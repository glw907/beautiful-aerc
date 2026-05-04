---
title: Block/Span sealed-sum markers drop kind enums
status: accepted
date: 2026-05-03
---

## Context

`internal/content/` defined `Block` and `Span` as sealed sum types
via marker methods (`blockType() blockKind`, `spanType() spanKind`)
returning private kind constants. The kind enums were never read —
every consumer discriminated via Go type switches on the concrete
type. The constants and their methods were ~30 LOC of pattern
ceremony with no consumer.

## Decision

Reduce the markers to no-arg, no-return methods:
`Block.isBlock()` and `Span.isSpan()`. Delete the `blockKind` /
`spanKind` types and their `kind*` constants. Naming follows the
`go/ast`-style sealed-sum convention (`exprNode()`, `stmtNode()`).

The `parse_test.go` table tests previously asserted block sequences
via `[]blockKind`; they now use `[]string` of concrete type names
produced inline with `fmt.Sprintf("%T", b)`.

## Consequences

`Block` / `Span` remain sealed to their package; new variants are
added by writing the type and an empty `isBlock()` / `isSpan()`
method. Discrimination at call sites is unchanged — type switches
already handled it. Zero behavior change at runtime; ~30 LOC
deleted.
