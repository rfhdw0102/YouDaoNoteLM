package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

type pptDOMExporter func(ctx context.Context, content, deckTitle string) ([]byte, error)

var defaultPPTDOMExporter pptDOMExporter = exportPPTWithDOMToPPTX

func exportPPTWithDefaultEngine(ctx context.Context, content, deckTitle string) ([]byte, error) {
	data, err := defaultPPTDOMExporter(ctx, content, deckTitle)
	if err == nil {
		return data, nil
	}

	logger.Warn("dom-to-pptx export failed, falling back to go exporter", zap.Error(err))
	return buildDynamicHTMLPPTX(content, deckTitle)
}

func exportPPTWithDOMToPPTX(ctx context.Context, content, deckTitle string) ([]byte, error) {
	scriptPath, err := resolvePPTDOMExporterScriptPath()
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "youdaonotelm-dom-ppt-export-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "input.html")
	outputPath := filepath.Join(tempDir, "output.pptx")
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		return nil, err
	}

	runCtx := ctx
	cancel := func() {}
	if _, ok := ctx.Deadline(); !ok {
		runCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
	}
	defer cancel()

	cmd := exec.CommandContext(runCtx, "node", scriptPath, inputPath, outputPath, deckTitle)
	cmd.Dir = filepath.Dir(scriptPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("run dom-to-pptx exporter: %s", message)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, err
	}
	if len(data) < 2 || string(data[:2]) != "PK" {
		return nil, bizerrors.New(bizerrors.CodeInternalServiceError, "dom-to-pptx exporter did not produce a pptx file")
	}
	return data, nil
}

func resolvePPTDOMExporterScriptPath() (string, error) {
	if fromEnv := strings.TrimSpace(os.Getenv("PPT_DOM_EXPORTER_SCRIPT")); fromEnv != "" {
		return fromEnv, nil
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", bizerrors.New(bizerrors.CodeInternalServiceError, "cannot resolve dom-to-pptx exporter path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	return filepath.Join(root, "ppt_exporter", "export_dom_to_pptx.mjs"), nil
}
