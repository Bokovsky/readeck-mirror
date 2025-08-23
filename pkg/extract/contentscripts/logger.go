// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"context"
	"log/slog"
	"strings"

	"github.com/dop251/goja"
)

// SetLogger sets the runtime's log entry.
func (vm *Runtime) SetLogger(logger *slog.Logger) {
	vm.ctx = withLogger(vm.ctx, logger)
}

// GetLogger returns the runtime's log entry or a default one
// when not set.
func (vm *Runtime) GetLogger() *slog.Logger {
	var logger *slog.Logger
	var ok bool
	if logger, ok = checkLogger(vm.ctx); !ok {
		logger = slog.Default()
	}

	// Add the script field when present
	if scriptName := vm.Get("__name__"); scriptName != nil {
		logger = logger.With(slog.String("script", scriptName.String()))
	}

	return logger
}

func (vm *Runtime) startConsole() error {
	console := vm.NewObject()
	if err := console.Set("debug", logFunc("debug", vm.GetLogger)); err != nil {
		return err
	}
	if err := console.Set("error", logFunc("error", vm.GetLogger)); err != nil {
		return err
	}
	if err := console.Set("info", logFunc("info", vm.GetLogger)); err != nil {
		return err
	}
	if err := console.Set("log", logFunc("log", vm.GetLogger)); err != nil {
		return err
	}
	if err := console.Set("warn", logFunc("warn", vm.GetLogger)); err != nil {
		return err
	}

	return vm.Set("console", console)
}

func logFunc(level string, getLogger func() *slog.Logger) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		msg := []string{}
		fields := []slog.Attr{}

		for _, x := range call.Arguments {
			if f, ok := x.(*goja.Object); ok {
				for _, k := range f.Keys() {
					fields = append(fields, valueToLogAttr(k, f.Get(k)))
				}
				continue
			}
			msg = append(msg, x.String())
		}

		lv := slog.LevelInfo
		switch level {
		case "debug":
			lv = slog.LevelDebug
		case "error":
			lv = slog.LevelError
		case "warn":
			lv = slog.LevelWarn
		}
		getLogger().LogAttrs(context.Background(), lv, strings.Join(msg, " "), fields...)

		return goja.Undefined()
	}
}

func valueToLogAttr(name string, v goja.Value) slog.Attr {
	switch x := v.Export().(type) {
	case int:
		return slog.Int(name, x)
	case float64:
		return slog.Float64(name, x)
	case float32:
		return slog.Float64(name, float64(x))
	case string:
		return slog.String(name, x)
	case bool:
		return slog.Bool(name, x)
	case []any:
		return slog.Any(name, x)
	case map[string]any:
		return slog.Any(name, x)
	default:
		return slog.Any(name, v.String())
	}
}
