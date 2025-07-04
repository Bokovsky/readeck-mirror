// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package srcset

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parse(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		want       SourceSet
		wantString string
	}{
		{
			name: "URL only",
			args: "logo-printer-friendly.svg",
			want: SourceSet{
				ImageSource{URL: "logo-printer-friendly.svg"},
			},
			wantString: "logo-printer-friendly.svg",
		},
		{
			name: "Parse URL & density",
			args: "image-1x.png 1x, image-2x.png 2x, image-3x.png 3x, image-4x.png 4x",
			want: SourceSet{
				ImageSource{URL: "image-1x.png", Density: 1},
				ImageSource{URL: "image-2x.png", Density: 2},
				ImageSource{URL: "image-3x.png", Density: 3},
				ImageSource{URL: "image-4x.png", Density: 4},
			},
			wantString: "image-1x.png 1x, image-2x.png 2x, image-3x.png 3x, image-4x.png 4x",
		},
		{
			name: "Parse URL & width - with line break whitespace",
			args: `elva-fairy-320w.jpg 320w,
			       elva-fairy-480w.jpg 480w,
			       elva-fairy-800w.jpg 800w`,
			want: SourceSet{
				ImageSource{URL: "elva-fairy-320w.jpg", Width: 320},
				ImageSource{URL: "elva-fairy-480w.jpg", Width: 480},
				ImageSource{URL: "elva-fairy-800w.jpg", Width: 800},
			},
			wantString: "elva-fairy-320w.jpg 320w, elva-fairy-480w.jpg 480w, elva-fairy-800w.jpg 800w",
		},
		{
			name: "Parse URL & height - with line break whitespace",
			args: `elva-fairy-320h.jpg 320h,
			       elva-fairy-480h.jpg 480h,
			       elva-fairy-800h.jpg 800h`,
			want: SourceSet{
				ImageSource{URL: "elva-fairy-320h.jpg", Height: 320},
				ImageSource{URL: "elva-fairy-480h.jpg", Height: 480},
				ImageSource{URL: "elva-fairy-800h.jpg", Height: 800},
			},
			wantString: "elva-fairy-320h.jpg 320h, elva-fairy-480h.jpg 480h, elva-fairy-800h.jpg 800h",
		},
		{
			name: "Invalid: Multiple densities",
			args: "test.png 1x 2x",
			want: SourceSet{},
		},
		{
			name: "Invalid: Density and width",
			args: "test.png 1x 200w",
			want: SourceSet{},
		},
		{
			name: "Invalid: negative width",
			args: "test.png -100w",
			want: SourceSet{},
		},
		{
			name: "Invalid: zero width",
			args: "test.png 0w",
			want: SourceSet{},
		},
		{
			name: "Invalid: None-number width",
			args: "test.png f55w",
			want: SourceSet{},
		},
		{
			name: "Invalid: negative height",
			args: "test.png -100h",
			want: SourceSet{},
		},
		{
			name: "Invalid: zero height",
			args: "test.png 0h",
			want: SourceSet{},
		},
		{
			name: "Invalid: multiple heights",
			args: "test.png 124h 234h",
			want: SourceSet{},
		},
		{
			name: "Invalid: negative density",
			args: "test.png -1.3x",
			want: SourceSet{},
		},

		{
			name: "Super funky",
			args: "data:,a ( , data:,b 1x, ), data:,c",
			want: SourceSet{
				ImageSource{URL: "data:,c"},
			},
			wantString: "data:,c",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			assert := require.New(t)
			p := Parse(test.args)
			assert.Equal(test.want, p)
			assert.Equal(test.wantString, p.String())
		})
	}
}
