// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"log/slog"

	. "github.com/phsym/console-slog" //nolint:revive,staticcheck
)

type consoleTheme struct {
	timestamp      ANSIMod
	source         ANSIMod
	message        ANSIMod
	messageDebug   ANSIMod
	attrKey        ANSIMod
	attrValue      ANSIMod
	attrValueError ANSIMod
	levelError     ANSIMod
	levelWarn      ANSIMod
	levelInfo      ANSIMod
	levelDebug     ANSIMod
}

func (t consoleTheme) Name() string            { return "" }
func (t consoleTheme) Timestamp() ANSIMod      { return t.timestamp }
func (t consoleTheme) Source() ANSIMod         { return t.source }
func (t consoleTheme) Message() ANSIMod        { return t.message }
func (t consoleTheme) MessageDebug() ANSIMod   { return t.messageDebug }
func (t consoleTheme) AttrKey() ANSIMod        { return t.attrKey }
func (t consoleTheme) AttrValue() ANSIMod      { return t.attrValue }
func (t consoleTheme) AttrValueError() ANSIMod { return t.attrValueError }
func (t consoleTheme) LevelError() ANSIMod     { return t.levelError }
func (t consoleTheme) LevelWarn() ANSIMod      { return t.levelWarn }
func (t consoleTheme) LevelInfo() ANSIMod      { return t.levelInfo }
func (t consoleTheme) LevelDebug() ANSIMod     { return t.levelDebug }
func (t consoleTheme) Level(level slog.Level) ANSIMod {
	switch {
	case level >= slog.LevelError:
		return t.LevelError()
	case level >= slog.LevelWarn:
		return t.LevelWarn()
	case level >= slog.LevelInfo:
		return t.LevelInfo()
	default:
		return t.LevelDebug()
	}
}

var stdLogTheme = consoleTheme{}

var devLogTheme = consoleTheme{
	timestamp:      ToANSICode(BrightBlack),
	source:         ToANSICode(Bold, BrightBlack),
	message:        ToANSICode(Bold),
	messageDebug:   ToANSICode(),
	attrKey:        ToANSICode(Cyan),
	attrValue:      ToANSICode(Faint),
	attrValueError: ToANSICode(Bold, Red),
	levelError:     ToANSICode(Bold, Red),
	levelWarn:      ToANSICode(Bold, Yellow),
	levelInfo:      ToANSICode(Bold, Green),
	levelDebug:     ToANSICode(Bold, BrightMagenta),
}
