// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestSpanLinkSimple(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	spanLink := span.Links().AppendEmpty()
	spanLink.SetTraceID(newTraceID())
	spanLink.SetSpanID(newSegmentID())

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.Equal(t, 1, len(segment.Links))
	assert.Equal(t, spanLink.SpanID().String(), *segment.Links[0].SpanID)
	assert.Equal(t, spanLink.TraceID().String(), *segment.Links[0].TraceID)
	assert.Equal(t, 0, len(segment.Links[0].Attributes))

	jsonStr, _ := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.True(t, strings.Contains(jsonStr, "links"))
	assert.False(t, strings.Contains(jsonStr, "attributes"))
	assert.True(t, strings.Contains(jsonStr, spanLink.TraceID().String()))
	assert.True(t, strings.Contains(jsonStr, spanLink.SpanID().String()))
}

func TestTwoSpanLinks(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	spanLink1 := span.Links().AppendEmpty()
	spanLink1.SetTraceID(newTraceID())
	spanLink1.SetSpanID(newSegmentID())
	spanLink1.Attributes().PutStr("myKey1", "ABC")

	spanLink2 := span.Links().AppendEmpty()
	spanLink2.SetTraceID(newTraceID())
	spanLink2.SetSpanID(newSegmentID())
	spanLink2.Attributes().PutStr("myKey2", "DEF")

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.Equal(t, 2, len(segment.Links))

	assert.Equal(t, spanLink1.SpanID().String(), *segment.Links[0].SpanID)
	assert.Equal(t, spanLink1.TraceID().String(), *segment.Links[0].TraceID)
	assert.Equal(t, 1, len(segment.Links[0].Attributes))
	assert.Equal(t, "ABC", segment.Links[0].Attributes["myKey1"])

	assert.Equal(t, spanLink2.SpanID().String(), *segment.Links[1].SpanID)
	assert.Equal(t, spanLink2.TraceID().String(), *segment.Links[1].TraceID)
	assert.Equal(t, 1, len(segment.Links[0].Attributes))
	assert.Equal(t, "DEF", segment.Links[1].Attributes["myKey2"])

	jsonStr, _ := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.True(t, strings.Contains(jsonStr, "attributes"))
	assert.True(t, strings.Contains(jsonStr, "links"))
	assert.True(t, strings.Contains(jsonStr, "myKey1"))
	assert.True(t, strings.Contains(jsonStr, "myKey2"))
	assert.True(t, strings.Contains(jsonStr, "ABC"))
	assert.True(t, strings.Contains(jsonStr, "DEF"))
}

func TestSpanLinkComplexAttributes(t *testing.T) {
	spanName := "ProcessingMessage"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	resource := constructDefaultResource()
	span := constructServerSpan(parentSpanID, spanName, ptrace.StatusCodeOk, "OK", attributes)

	spanLink := span.Links().AppendEmpty()
	spanLink.SetTraceID(newTraceID())
	spanLink.SetSpanID(newSegmentID())
	spanLink.Attributes().PutStr("myKey1", "myValue")
	spanLink.Attributes().PutBool("myKey2", true)
	spanLink.Attributes().PutInt("myKey3", 112233)
	spanLink.Attributes().PutDouble("myKey4", 3.1415)

	var slice1 = spanLink.Attributes().PutEmptySlice("myKey5")
	slice1.AppendEmpty().SetStr("apple")
	slice1.AppendEmpty().SetStr("pear")
	slice1.AppendEmpty().SetStr("banana")

	var slice2 = spanLink.Attributes().PutEmptySlice("myKey6")
	slice2.AppendEmpty().SetBool(true)
	slice2.AppendEmpty().SetBool(false)
	slice2.AppendEmpty().SetBool(false)
	slice2.AppendEmpty().SetBool(true)

	var slice3 = spanLink.Attributes().PutEmptySlice("myKey7")
	slice3.AppendEmpty().SetInt(1234)
	slice3.AppendEmpty().SetInt(5678)
	slice3.AppendEmpty().SetInt(9012)

	var slice4 = spanLink.Attributes().PutEmptySlice("myKey8")
	slice4.AppendEmpty().SetDouble(2.718)
	slice4.AppendEmpty().SetDouble(1.618)

	segment, _ := MakeSegment(span, resource, nil, false, nil)

	assert.Equal(t, 1, len(segment.Links))
	assert.Equal(t, spanLink.SpanID().String(), *segment.Links[0].SpanID)
	assert.Equal(t, spanLink.TraceID().String(), *segment.Links[0].TraceID)
	assert.Equal(t, 8, len(segment.Links[0].Attributes))

	assert.Equal(t, "myValue", segment.Links[0].Attributes["myKey1"])
	assert.Equal(t, true, segment.Links[0].Attributes["myKey2"])
	assert.Equal(t, int64(112233), segment.Links[0].Attributes["myKey3"])
	assert.Equal(t, 3.1415, segment.Links[0].Attributes["myKey4"])

	assert.Equal(t, "apple", segment.Links[0].Attributes["myKey5"].([]interface{})[0])
	assert.Equal(t, "pear", segment.Links[0].Attributes["myKey5"].([]interface{})[1])
	assert.Equal(t, "banana", segment.Links[0].Attributes["myKey5"].([]interface{})[2])

	assert.Equal(t, true, segment.Links[0].Attributes["myKey6"].([]interface{})[0])
	assert.Equal(t, false, segment.Links[0].Attributes["myKey6"].([]interface{})[1])
	assert.Equal(t, false, segment.Links[0].Attributes["myKey6"].([]interface{})[2])
	assert.Equal(t, true, segment.Links[0].Attributes["myKey6"].([]interface{})[0])

	assert.Equal(t, int64(1234), segment.Links[0].Attributes["myKey7"].([]interface{})[0])
	assert.Equal(t, int64(5678), segment.Links[0].Attributes["myKey7"].([]interface{})[1])
	assert.Equal(t, int64(9012), segment.Links[0].Attributes["myKey7"].([]interface{})[2])

	assert.Equal(t, 2.718, segment.Links[0].Attributes["myKey8"].([]interface{})[0])
	assert.Equal(t, 1.618, segment.Links[0].Attributes["myKey8"].([]interface{})[1])

	jsonStr, _ := MakeSegmentDocumentString(span, resource, nil, false, nil)

	assert.True(t, strings.Contains(jsonStr, "links"))

	assert.True(t, strings.Contains(jsonStr, "myKey1"))
	assert.True(t, strings.Contains(jsonStr, "myValue"))

	assert.True(t, strings.Contains(jsonStr, "myKey2"))
	assert.True(t, strings.Contains(jsonStr, "true"))

	assert.True(t, strings.Contains(jsonStr, "myKey3"))
	assert.True(t, strings.Contains(jsonStr, "112233"))

	assert.True(t, strings.Contains(jsonStr, "myKey4"))
	assert.True(t, strings.Contains(jsonStr, "3.1415"))

	assert.True(t, strings.Contains(jsonStr, "myKey5"))
	assert.True(t, strings.Contains(jsonStr, "apple"))
	assert.True(t, strings.Contains(jsonStr, "pear"))
	assert.True(t, strings.Contains(jsonStr, "banana"))

	assert.True(t, strings.Contains(jsonStr, "myKey6"))
	assert.True(t, strings.Contains(jsonStr, "false"))

	assert.True(t, strings.Contains(jsonStr, "myKey7"))
	assert.True(t, strings.Contains(jsonStr, "1234"))
	assert.True(t, strings.Contains(jsonStr, "5678"))
	assert.True(t, strings.Contains(jsonStr, "9012"))

	assert.True(t, strings.Contains(jsonStr, "myKey8"))
	assert.True(t, strings.Contains(jsonStr, "2.718"))
	assert.True(t, strings.Contains(jsonStr, "1.618"))
}
