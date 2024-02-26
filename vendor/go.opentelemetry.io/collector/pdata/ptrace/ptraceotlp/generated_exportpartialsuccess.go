// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Code generated by "pdata/internal/cmd/pdatagen/main.go". DO NOT EDIT.
// To regenerate this file run "make genpdata".

package ptraceotlp

import (
	"go.opentelemetry.io/collector/pdata/internal"
	otlpcollectortrace "go.opentelemetry.io/collector/pdata/internal/data/protogen/collector/trace/v1"
)

// ExportPartialSuccess represents the details of a partially successful export request.
//
// This is a reference type, if passed by value and callee modifies it the
// caller will see the modification.
//
// Must use NewExportPartialSuccess function to create new instances.
// Important: zero-initialized instance is not valid for use.
type ExportPartialSuccess struct {
	orig  *otlpcollectortrace.ExportTracePartialSuccess
	state *internal.State
}

func newExportPartialSuccess(orig *otlpcollectortrace.ExportTracePartialSuccess, state *internal.State) ExportPartialSuccess {
	return ExportPartialSuccess{orig: orig, state: state}
}

// NewExportPartialSuccess creates a new empty ExportPartialSuccess.
//
// This must be used only in testing code. Users should use "AppendEmpty" when part of a Slice,
// OR directly access the member if this is embedded in another struct.
func NewExportPartialSuccess() ExportPartialSuccess {
	state := internal.StateMutable
	return newExportPartialSuccess(&otlpcollectortrace.ExportTracePartialSuccess{}, &state)
}

// MoveTo moves all properties from the current struct overriding the destination and
// resetting the current instance to its zero value
func (ms ExportPartialSuccess) MoveTo(dest ExportPartialSuccess) {
	ms.state.AssertMutable()
	dest.state.AssertMutable()
	*dest.orig = *ms.orig
	*ms.orig = otlpcollectortrace.ExportTracePartialSuccess{}
}

// RejectedSpans returns the rejectedspans associated with this ExportPartialSuccess.
func (ms ExportPartialSuccess) RejectedSpans() int64 {
	return ms.orig.RejectedSpans
}

// SetRejectedSpans replaces the rejectedspans associated with this ExportPartialSuccess.
func (ms ExportPartialSuccess) SetRejectedSpans(v int64) {
	ms.state.AssertMutable()
	ms.orig.RejectedSpans = v
}

// ErrorMessage returns the errormessage associated with this ExportPartialSuccess.
func (ms ExportPartialSuccess) ErrorMessage() string {
	return ms.orig.ErrorMessage
}

// SetErrorMessage replaces the errormessage associated with this ExportPartialSuccess.
func (ms ExportPartialSuccess) SetErrorMessage(v string) {
	ms.state.AssertMutable()
	ms.orig.ErrorMessage = v
}

// CopyTo copies all properties from the current struct overriding the destination.
func (ms ExportPartialSuccess) CopyTo(dest ExportPartialSuccess) {
	dest.state.AssertMutable()
	dest.SetRejectedSpans(ms.RejectedSpans())
	dest.SetErrorMessage(ms.ErrorMessage())
}