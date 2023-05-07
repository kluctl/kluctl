package commands

import (
	"bytes"
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/diff"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io"
	"os"
	"strings"
)

func formatCommandResultText(cr *result.CommandResult, short bool) string {
	buf := bytes.NewBuffer(nil)

	if len(cr.Warnings) != 0 {
		buf.WriteString("\nWarnings:\n")
		prettyErrors(buf, cr.Warnings)
	}

	var newObjects []k8s.ObjectRef
	var changedObjects []k8s.ObjectRef
	var deletedObjects []k8s.ObjectRef
	var orphanObjects []k8s.ObjectRef
	var appliedHookObjects []k8s.ObjectRef

	for _, o := range cr.Objects {
		if o.New {
			newObjects = append(newObjects, o.Ref)
		}
		if len(o.Changes) != 0 {
			changedObjects = append(changedObjects, o.Ref)
		}
		if o.Deleted {
			deletedObjects = append(deletedObjects, o.Ref)
		}
		if o.Orphan {
			orphanObjects = append(orphanObjects, o.Ref)
		}
		if o.Hook {
			appliedHookObjects = append(appliedHookObjects, o.Ref)
		}
	}

	if len(newObjects) != 0 {
		buf.WriteString("\nNew objects:\n")
		prettyObjectRefs(buf, newObjects)
	}
	if len(changedObjects) != 0 {
		buf.WriteString("\nChanged objects:\n")
		prettyObjectRefs(buf, changedObjects)

		if !short {
			buf.WriteString("\n")
			for i, o := range cr.Objects {
				if len(o.Changes) == 0 {
					continue
				}
				if i != 0 {
					buf.WriteString("\n")
				}
				prettyChanges(buf, o.Ref, o.Changes)
			}
		}
	}

	if len(deletedObjects) != 0 {
		buf.WriteString("\nDeleted objects:\n")
		prettyObjectRefs(buf, deletedObjects)
	}

	if len(appliedHookObjects) != 0 {
		buf.WriteString("\nApplied hooks:\n")
		prettyObjectRefs(buf, appliedHookObjects)
	}
	if len(orphanObjects) != 0 {
		buf.WriteString("\nOrphan objects:\n")
		prettyObjectRefs(buf, orphanObjects)
	}

	if len(cr.Errors) != 0 {
		buf.WriteString("\nErrors:\n")
		prettyErrors(buf, cr.Errors)
	}

	return buf.String()
}

func prettyObjectRefs(buf io.StringWriter, refs []k8s.ObjectRef) {
	for _, ref := range refs {
		_, _ = buf.WriteString(fmt.Sprintf("  %s\n", ref.String()))
	}
}

func prettyErrors(buf io.StringWriter, errors []result.DeploymentError) {
	for _, e := range errors {
		prefix := ""
		if s := e.Ref.String(); s != "" {
			prefix = fmt.Sprintf("%s: ", s)
		}
		_, _ = buf.WriteString(fmt.Sprintf("  %s%s\n", prefix, e.Message))
	}
}

func prettyChanges(buf io.StringWriter, ref k8s.ObjectRef, changes []result.Change) {
	_, _ = buf.WriteString(fmt.Sprintf("Diff for object %s\n", ref.String()))

	var t utils.PrettyTable
	t.AddRow("Path", "Diff")

	for _, c := range changes {
		t.AddRow(c.JsonPath, c.UnifiedDiff)
	}
	s := t.Render([]int{60})
	_, _ = buf.WriteString(s)
}

func formatCommandResultYaml(cr *result.CommandResult) (string, error) {
	b, err := yaml.WriteYamlString(cr.ToCompacted())
	if err != nil {
		return "", err
	}
	return b, nil
}

func formatCommandResult(cr *result.CommandResult, format string, short bool) (string, error) {
	switch format {
	case "text":
		return formatCommandResultText(cr, short), nil
	case "yaml":
		return formatCommandResultYaml(cr)
	default:
		return "", fmt.Errorf("invalid format: %s", format)
	}
}

func prettyValidationResults(buf io.StringWriter, results []result.ValidateResultEntry) {
	var t utils.PrettyTable
	t.AddRow("Object", "Message")

	for _, e := range results {
		t.AddRow(e.Ref.String(), e.Message)
	}
	s := t.Render([]int{60})
	_, _ = buf.WriteString(s)
}

func formatValidateResultText(vr *result.ValidateResult) string {
	buf := bytes.NewBuffer(nil)

	if len(vr.Warnings) != 0 {
		buf.WriteString("\nValidation Warnings:\n")
		prettyErrors(buf, vr.Warnings)
	}

	if len(vr.Errors) != 0 {
		if buf.Len() != 0 {
			buf.WriteString("\n")
		}
		buf.WriteString("Validation Errors:\n")
		prettyErrors(buf, vr.Errors)
	}

	if len(vr.Results) != 0 {
		if buf.Len() != 0 {
			buf.WriteString("\n")
		}
		buf.WriteString("Results:\n")
		prettyValidationResults(buf, vr.Results)
	}

	if len(vr.Drift) != 0 {
		if buf.Len() != 0 {
			buf.WriteString("\n")
		}
		buf.WriteString("Drift:\n")
		for i, o := range vr.Drift {
			if len(o.Changes) == 0 {
				continue
			}
			if i != 0 {
				buf.WriteString("\n")
			}
			prettyChanges(buf, o.Ref, o.Changes)
		}
	}

	return buf.String()
}

func formatValidateResultYaml(vr *result.ValidateResult) (string, error) {
	b, err := yaml.WriteYamlString(vr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func formatValidateResult(vr *result.ValidateResult, format string) (string, error) {
	switch format {
	case "text":
		return formatValidateResultText(vr), nil
	case "yaml":
		return formatValidateResultYaml(vr)
	default:
		return "", fmt.Errorf("invalid validation result format: %s", format)
	}
}

func outputHelper(ctx context.Context, output []string, cb func(format string) (string, error)) error {
	if len(output) == 0 {
		output = []string{"text"}
	}
	for _, o := range output {
		s := strings.SplitN(o, "=", 2)
		format := s[0]
		var path *string
		if len(s) > 1 {
			path = &s[1]
		}
		r, err := cb(format)
		if err != nil {
			return err
		}

		err = outputResult(ctx, path, r)
		if err != nil {
			return err
		}
	}
	return nil
}

func outputCommandResult(ctx *commandCtx, flags args.OutputFormatFlags, cr *result.CommandResult, writeToResultStore bool) error {
	status.Flush(ctx.ctx)

	if !flags.NoObfuscate {
		var obfuscator diff.Obfuscator
		err := obfuscator.ObfuscateResult(cr)
		if err != nil {
			return err
		}
	}

	var resultStoreErr error
	if writeToResultStore && ctx.resultStore != nil {
		s := status.Start(ctx.ctx, "Writing command result")
		defer s.Failed()
		resultStoreErr = ctx.resultStore.WriteCommandResult(cr)
		if resultStoreErr != nil {
			s.FailedWithMessage("Failed to write result to result store: %s", resultStoreErr.Error())
		} else {
			s.Success()
		}
	}

	err := outputHelper(ctx.ctx, flags.OutputFormat, func(format string) (string, error) {
		return formatCommandResult(cr, format, flags.ShortOutput)
	})
	if err == nil && resultStoreErr != nil {
		return resultStoreErr
	}
	return err
}

func outputValidateResult(ctx context.Context, output []string, vr *result.ValidateResult) error {
	status.Flush(ctx)

	return outputHelper(ctx, output, func(format string) (string, error) {
		return formatValidateResult(vr, format)
	})
}

func outputYamlResult(ctx context.Context, output []string, result interface{}, multiDoc bool) error {
	status.Flush(ctx)

	if len(output) == 0 {
		output = []string{"-"}
	}
	var s string
	if multiDoc {
		l, ok := result.([]interface{})
		if !ok {
			return fmt.Errorf("object is not a list")
		}
		x, err := yaml.WriteYamlAllString(l)
		if err != nil {
			return err
		}
		s = x
	} else {
		x, err := yaml.WriteYamlString(result)
		if err != nil {
			return err
		}
		s = x
	}
	for _, path := range output {
		err := outputResult(ctx, &path, s)
		if err != nil {
			return err
		}
	}
	return nil
}

func outputResult(ctx context.Context, f *string, result string) error {
	var w io.Writer
	w = getStdout(ctx)
	if f != nil && *f != "-" {
		f, err := os.Create(*f)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	_, err := w.Write([]byte(result))
	return err
}
