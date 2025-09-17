// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"github.com/dop251/goja"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/pkg/bleach"
	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
	"codeberg.org/readeck/readeck/pkg/xml2map"
)

type processMessageProxy struct {
	vm *Runtime
}

func registerExported(vm *Runtime) (err error) {
	var p *goja.Object
	if p, err = newProcessMessageProxy(vm); err != nil {
		return
	}
	if err = vm.Set("$", p); err != nil {
		return
	}
	if err = vm.Set("unescapeURL", unescapeURL); err != nil {
		return
	}
	if err = vm.Set("decodeXML", decodeXML); err != nil {
		return
	}
	if err = vm.Set("escapeHTML", html.EscapeString); err != nil {
		return
	}
	if err = vm.Set("unescapeHTML", html.UnescapeString); err != nil {
		return
	}
	return
}

// SetProcessMessage adds an extract.ProcessMessage to the content script context.
func (vm *Runtime) SetProcessMessage(m *extract.ProcessMessage) {
	vm.ctx = withMessage(vm.ctx, m)
}

func (vm *Runtime) getProcessMessage() *extract.ProcessMessage {
	if m, ok := checkMessage(vm.ctx); ok {
		return m
	}
	return nil
}

func newProcessMessageProxy(vm *Runtime) (*goja.Object, error) {
	p := &processMessageProxy{vm: vm}

	obj := vm.NewObject()
	if err := obj.Set("meta", newDropMetaProxyObj(vm)); err != nil {
		return nil, err
	}
	if err := obj.DefineAccessorProperty(
		"properties", vm.ToValue(p.getProperties), nil,
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	); err != nil {
		return nil, err
	}
	if err := obj.DefineAccessorProperty(
		"domain", vm.ToValue(p.getDomain), nil,
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	); err != nil {
		return nil, err
	}
	if err := obj.DefineAccessorProperty(
		"host", vm.ToValue(p.getHost), nil,
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	); err != nil {
		return nil, err
	}
	if err := obj.DefineAccessorProperty(
		"url", vm.ToValue(p.getURL), nil,
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	); err != nil {
		return nil, err
	}
	if err := obj.DefineAccessorProperty(
		"authors", vm.ToValue(p.getAuthors), vm.ToValue(p.setAuthors),
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	); err != nil {
		return nil, err
	}
	if err := obj.DefineAccessorProperty(
		"description", vm.ToValue(p.getDescription), vm.ToValue(p.setDescription),
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	); err != nil {
		return nil, err
	}
	if err := obj.DefineAccessorProperty(
		"site", vm.ToValue(p.getSiteName), vm.ToValue(p.setSiteName),
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	); err != nil {
		return nil, err
	}
	if err := obj.DefineAccessorProperty(
		"title", vm.ToValue(p.getTitle), vm.ToValue(p.setTitle),
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	); err != nil {
		return nil, err
	}
	if err := obj.DefineAccessorProperty(
		"type", vm.ToValue(p.getType), vm.ToValue(p.setType),
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	); err != nil {
		return nil, err
	}
	if err := obj.DefineAccessorProperty(
		"readability", vm.ToValue(p.getReadability), vm.ToValue(p.setReadability),
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	); err != nil {
		return nil, err
	}
	if err := obj.DefineAccessorProperty(
		"html", nil, vm.ToValue(p.setHTML),
		goja.FLAG_FALSE, goja.FLAG_FALSE,
	); err != nil {
		return nil, err
	}
	if err := obj.Set("overrideConfig", p.overrideConfig); err != nil {
		return nil, err
	}
	return obj, nil
}

func (p *processMessageProxy) getDrop() *extract.Drop {
	if pm := p.vm.getProcessMessage(); pm != nil {
		return pm.Extractor.Drop()
	}
	panic(p.vm.ToValue("no extractor"))
}

func (p *processMessageProxy) getProcessMessage() *extract.ProcessMessage {
	if pm := p.vm.getProcessMessage(); pm != nil {
		return pm
	}
	panic(p.vm.ToValue("no extractor"))
}

func (p *processMessageProxy) getProperties() map[string]any {
	return p.getDrop().Properties
}

func (p *processMessageProxy) getDomain() string {
	return p.getDrop().Domain
}

func (p *processMessageProxy) getHost() string {
	return p.getDrop().URL.Hostname()
}

func (p *processMessageProxy) getURL() string {
	return p.getDrop().URL.String()
}

func (p *processMessageProxy) getAuthors() []string {
	return p.getDrop().Authors
}

func (p *processMessageProxy) setAuthors(names ...string) {
	p.getDrop().Authors = names
	p.vm.GetLogger().Debug("set property", slog.Any("authors", names))
}

func (p *processMessageProxy) getDescription() string {
	return p.getDrop().Description
}

func (p *processMessageProxy) setDescription(val string) {
	p.getDrop().Description = val
	p.vm.GetLogger().Debug("set property", slog.String("description", val))
}

func (p *processMessageProxy) getSiteName() string {
	return p.getDrop().Site
}

func (p *processMessageProxy) setSiteName(val string) {
	p.getDrop().Site = val
	p.vm.GetLogger().Debug("set property", slog.String("site", val))
}

func (p *processMessageProxy) getTitle() string {
	return p.getDrop().Title
}

func (p *processMessageProxy) setTitle(val string) {
	p.getDrop().Title = val
	p.vm.GetLogger().Debug("set property", slog.String("title", val))
}

func (p *processMessageProxy) getType() string {
	return p.getDrop().DocumentType
}

func (p *processMessageProxy) setType(val string) error {
	if !slices.Contains([]string{"article", "photo", "video"}, val) {
		return fmt.Errorf(`"%s" is not a valid type`, val)
	}
	p.getDrop().DocumentType = val
	p.vm.GetLogger().Debug("set property", slog.String("document_type", val))
	return nil
}

func (p *processMessageProxy) setHTML(val string) error {
	node, err := html.Parse(strings.NewReader(bleach.SanitizeString(val)))
	if err != nil {
		return err
	}
	p.getProcessMessage().Dom = node
	p.getDrop().ContentType = "text/html"
	p.vm.GetLogger().Debug("set property",
		slog.String("html", val[0:min(50, len(val))]+"..."),
	)
	return nil
}

func (p *processMessageProxy) getReadability() bool {
	enabled, _ := contents.IsReadabilityEnabled(p.getProcessMessage().Extractor)
	return enabled
}

func (p *processMessageProxy) setReadability(val bool) {
	contents.EnableReadability(p.getProcessMessage().Extractor, val)
}

func (p *processMessageProxy) overrideConfig(cfg *SiteConfig, src string) error {
	u, err := url.Parse(src)
	if err != nil {
		return err
	}
	newConfig, err := NewConfigForURL(SiteConfigFiles, u)
	if err != nil {
		return err
	}

	*cfg = *newConfig
	p.vm.GetLogger().Debug("site configuration override", slog.Any("files", cfg.files))
	return nil
}

type dropMetaProxy struct {
	vm *Runtime
}

func newDropMetaProxyObj(vm *Runtime) *goja.Object {
	return vm.NewDynamicObject(&dropMetaProxy{vm})
}

func (m *dropMetaProxy) meta() extract.DropMeta {
	if pm := m.vm.getProcessMessage(); pm != nil {
		return pm.Extractor.Drop().Meta
	}
	panic(m.vm.ToValue("no extractor"))
}

func (m *dropMetaProxy) Get(key string) goja.Value {
	return m.vm.ToValue(m.meta()[key])
}

func (m *dropMetaProxy) Set(key string, val goja.Value) bool {
	var v []string
	switch sv := val.Export().(type) {
	case string:
		v = []string{sv}
	default:
		if err := m.vm.ExportTo(val, &v); err != nil {
			return false
		}
	}
	m.meta()[key] = v
	m.vm.GetLogger().Debug("set meta", slog.Any(key, v))
	return true
}

func (m *dropMetaProxy) Has(key string) bool {
	_, ok := m.meta()[key]
	return ok
}

func (m *dropMetaProxy) Delete(key string) bool {
	delete(m.meta(), key)
	return true
}

func (m *dropMetaProxy) Keys() []string {
	keys := []string{}
	for k := range m.meta() {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func unescapeURL(src string) (string, error) {
	v, err := url.Parse(src)
	if err != nil {
		return "", err
	}

	if v.RawQuery, err = url.QueryUnescape(v.RawQuery); err != nil {
		return "", err
	}
	if v.Path, err = url.PathUnescape(v.Path); err != nil {
		return "", err
	}

	return v.String(), nil
}

func decodeXML(src []byte) (map[string]any, error) {
	return xml2map.NewDecoder(bytes.NewBuffer(src)).Decode()
}
