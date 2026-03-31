package docker

import (
	"bytes"
	"strings"
	"testing"
)

func TestStreamBuildOutput_StreamMessages(t *testing.T) {
	input := `{"stream":"Step 1/5 : FROM ubuntu:22.04\n"}
{"stream":" ---> abc123\n"}
{"stream":"Step 2/5 : RUN apt-get update\n"}
{"stream":"some build output\n"}
`
	var out bytes.Buffer
	err := streamBuildOutput(strings.NewReader(input), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := out.String()
	if !strings.Contains(result, "Step 1/5") {
		t.Error("should contain Step lines")
	}
	if !strings.Contains(result, "abc123") {
		t.Error("should show intermediate hash lines")
	}
	if !strings.Contains(result, "some build output") {
		t.Error("should show regular build output")
	}
}

func TestStreamBuildOutput_StepColoring(t *testing.T) {
	input := `{"stream":"Step 1/3 : FROM ubuntu\n"}
`
	var out bytes.Buffer
	_ = streamBuildOutput(strings.NewReader(input), &out)
	result := out.String()
	if !strings.Contains(result, ansiCyan) {
		t.Error("Step lines should be colored cyan")
	}
}

func TestStreamBuildOutput_SuccessColoring(t *testing.T) {
	input := `{"stream":"Successfully built abc123\n"}
{"stream":"Successfully tagged myimage:latest\n"}
`
	var out bytes.Buffer
	_ = streamBuildOutput(strings.NewReader(input), &out)
	result := out.String()
	if !strings.Contains(result, ansiGreen) {
		t.Error("success lines should be colored green")
	}
}

func TestStreamBuildOutput_ErrorMessages(t *testing.T) {
	input := `{"error":"something went wrong","errorDetail":{"message":"something went wrong"}}
`
	var out bytes.Buffer
	err := streamBuildOutput(strings.NewReader(input), &out)

	result := out.String()
	if !strings.Contains(result, "something went wrong") {
		t.Error("should show error message")
	}
	if !strings.Contains(result, ansiRed) {
		t.Error("error should be colored red")
	}
	if err == nil {
		t.Error("should return error when build fails")
	}
}

func TestStreamBuildOutput_PullStatus(t *testing.T) {
	input := `{"status":"Pulling from library/ubuntu","id":""}
{"status":"Downloading","progress":"[====>      ] 10MB/50MB","id":"abc123"}
{"status":"Digest: sha256:abcdef"}
`
	var out bytes.Buffer
	_ = streamBuildOutput(strings.NewReader(input), &out)

	result := out.String()
	if !strings.Contains(result, "Pulling from library/ubuntu") {
		t.Error("should show pull status")
	}
	if !strings.Contains(result, "Digest:") {
		t.Error("should show digest line")
	}
}

func TestStreamBuildOutput_NonJSON(t *testing.T) {
	input := "this is not json\n"
	var out bytes.Buffer
	_ = streamBuildOutput(strings.NewReader(input), &out)
	if !strings.Contains(out.String(), "this is not json") {
		t.Error("should pass through non-JSON lines")
	}
}

func TestStreamBuildOutput_PreservesNewlines(t *testing.T) {
	input := `{"stream":"line one\n"}
{"stream":"line two\n"}
`
	var out bytes.Buffer
	_ = streamBuildOutput(strings.NewReader(input), &out)
	result := out.String()
	if !strings.Contains(result, "line one\n") {
		t.Error("should preserve newlines from stream")
	}
	if !strings.Contains(result, "line two\n") {
		t.Error("should preserve newlines from stream")
	}
}
