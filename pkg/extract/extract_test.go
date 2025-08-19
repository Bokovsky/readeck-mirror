// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract_test

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
	. "codeberg.org/readeck/readeck/pkg/extract/testing" //revive:disable:dot-imports
)

func mustParse(src string) *url.URL {
	u, err := url.Parse(src)
	if err != nil {
		panic(err)
	}
	return u
}

func TestErrors(t *testing.T) {
	errlist := extract.Error{errors.New("err1"), errors.New("err2")}
	require.Equal(t, "err1, err2", errlist.Error())
}

func TestURLList(t *testing.T) {
	assert := require.New(t)
	list := extract.URLList{}
	list.Add(mustParse("http://example.net/main"))

	assert.True(list.IsPresent(mustParse("http://example.net/main")))
	assert.False(list.IsPresent(mustParse("http://example.org/")))

	list.Add(mustParse("http://example.org/"))
	assert.True(list.IsPresent(mustParse("http://example.org/")))
}

func TestExtractor(t *testing.T) {
	t.Run("new with error", func(t *testing.T) {
		ex, err := extract.New("http://example.net/\b0x7f", nil)
		require.Nil(t, ex)
		require.Contains(t, err.Error(), "invalid control")
	})

	t.Run("new", func(t *testing.T) {
		assert := require.New(t)
		ex, _ := extract.New("http://example.net/#frag", nil)
		assert.Equal("http://example.net/", ex.URL.String())
		assert.Len(ex.Drops(), 1)

		drop := ex.Drop()
		assert.Equal(drop, ex.Drops()[0])
		assert.Equal("http://example.net/", drop.URL.String())

		ex.AddError(errors.New("err1"))
		assert.Equal("err1", ex.Errors().Error())
	})

	t.Run("drops", func(t *testing.T) {
		assert := require.New(t)
		ex := extract.Extractor{}
		assert.Nil(ex.Drop())

		ex.AddDrop(mustParse("http://example.net/"))
		assert.Equal("http://example.net/", ex.Drop().URL.String())

		err := ex.ReplaceDrop(mustParse("http://example.net/new"))
		assert.NoError(err)
		assert.Equal("http://example.net/new", ex.Drop().URL.String())

		ex.AddDrop(mustParse("http://example.net/page2"))
		err = ex.ReplaceDrop(mustParse("http://example.net/page1"))
		assert.Equal(
			"cannot replace a drop when there are more that one",
			err.Error(),
		)
	})
}

func TestExtractorRun(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/404", httpmock.NewJsonResponderOrPanic(404, ""))
	httpmock.RegisterResponder("GET", "/page1", NewHTMLResponder(200, "html/ex1.html"))
	httpmock.RegisterResponder("GET", `=~^/loop/\d+`, NewHTMLResponder(200, "html/ex1.html"))

	ctxBodyKey := &struct{}{}

	p1 := func(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
		if m.Step() != extract.StepBody {
			return next
		}

		m.Extractor.Drops()[m.Position()].Body = []byte("test")
		return next
	}

	p2a := func(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
		if m.Step() != extract.StepBody {
			return next
		}

		m.Extractor.Context = context.WithValue(m.Extractor.Context, ctxBodyKey, []byte("@@body@@"))

		return next
	}

	p2b := func(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
		if m.Step() != extract.StepBody {
			return next
		}

		m.Extractor.Drops()[m.Position()].Body = m.Extractor.Context.Value(ctxBodyKey).([]byte)

		return next
	}

	p3 := func(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
		if m.Step() != extract.StepBody {
			return next
		}

		if m.Position() == 0 {
			m.Extractor.AddDrop(mustParse("http://example.org/page1"))
		}
		if m.Position() == 1 {
			m.Extractor.AddDrop(mustParse("http://example.net/page1"))
		}
		if m.Position() > 2 {
			// That will never happen
			panic("We should never loop")
		}
		return next
	}

	loopProcessor := func() extract.Processor {
		// Simulates the case of a page managing to force a processor into infinite
		// redirections to a new content page.
		iterations := 200
		i := 0
		return func(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
			if m.Step() != extract.StepDom {
				return next
			}

			if i >= iterations {
				return next
			}

			i++
			u, _ := m.Extractor.Drop().URL.Parse(strconv.Itoa(i))

			_ = m.Extractor.ReplaceDrop(u)
			m.ResetPosition()

			return next
		}
	}

	tooManyDropProcessor := func() extract.Processor {
		iterations := 200
		i := 0
		return func(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
			if m.Step() != extract.StepFinish {
				return next
			}
			if i >= iterations {
				return next
			}

			i++
			u, _ := m.Extractor.Drop().URL.Parse(strconv.Itoa(i))
			m.Extractor.AddDrop(u)
			return next
		}
	}

	t.Run("simple", func(t *testing.T) {
		assert := require.New(t)
		ex, _ := extract.New("http://example.net/page1", nil)
		ex.Run()
		assert.Empty(ex.Errors())
		assert.Contains(string(ex.Drop().Body), "Otters have long, slim bodies")
	})

	t.Run("load error", func(t *testing.T) {
		assert := require.New(t)
		ex, _ := extract.New("http://example.net/404", nil)
		ex.Run()
		assert.Len(ex.Errors(), 1)
		assert.Equal("cannot load resource", ex.Errors().Error())
	})

	t.Run("process body", func(t *testing.T) {
		assert := require.New(t)
		ex, _ := extract.New("http://example.net/page1", nil)
		ex.AddProcessors(p1)
		ex.Run()
		assert.Empty(ex.Errors())
		assert.Equal("test", string(ex.Drop().Body))
	})

	t.Run("process passing values", func(t *testing.T) {
		assert := require.New(t)
		ex, _ := extract.New("http://example.net/page1", nil)
		ex.AddProcessors(p2a, p2b)
		ex.Run()
		assert.Empty(ex.Errors())
		assert.Equal("@@body@@", string(ex.Drop().Body))
	})

	t.Run("process add drop", func(t *testing.T) {
		assert := require.New(t)
		ex, _ := extract.New("http://example.net/page1", nil)
		ex.AddProcessors(p3)
		ex.Run()
		assert.Empty(ex.Errors())
		assert.Len(ex.Drops(), 3)
		assert.Equal("http://example.net/page1", ex.Drops()[0].URL.String())
		assert.Equal("http://example.org/page1", ex.Drops()[1].URL.String())
	})

	t.Run("too many redirects", func(t *testing.T) {
		assert := require.New(t)
		ex, _ := extract.New("http://example.net/loop/0", nil)
		ex.AddProcessors(loopProcessor())
		ex.Run()
		assert.Len(ex.Errors(), 1)
		assert.Equal("operation canceled", ex.Errors().Error())
		assert.Equal(
			`[ERRO] operation canceled step.id="3" step.name="dom" err="too many redirects"`,
			ex.Logs[len(ex.Logs)-2],
		)
	})

	t.Run("too many pages", func(t *testing.T) {
		assert := require.New(t)
		ex, _ := extract.New("http://example.net/loop/0", nil)
		ex.AddProcessors(tooManyDropProcessor())
		ex.Run()
		assert.Len(ex.Errors(), 1)
		assert.Equal("operation canceled", ex.Errors().Error())
		assert.Equal(
			`[ERRO] operation canceled step.id="4" step.name="finish" err="too many pages"`,
			ex.Logs[len(ex.Logs)-2],
		)
	})
}
