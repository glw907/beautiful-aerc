// SPDX-License-Identifier: MIT

// Package term provides terminal capability detection used at poplar
// startup: Nerd Font installation discovery (HasNerdFont) and DSR/CPR
// probe of an SPUA-A glyph's rendered cell width (MeasureSPUACells).
//
// The package is consumed only by cmd/poplar; internal/ui receives the
// resolved values via constructor injection (IconSet, spuaCellWidth).
package term
