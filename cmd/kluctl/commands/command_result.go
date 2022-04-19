package commands

import (
	"bytes"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io"
	"os"
	"strings"
)

func formatCommandResultText(cr *types.CommandResult) string {
	buf := bytes.NewBuffer(nil)

	if len(cr.Warnings) != 0 {
		buf.WriteString("\nWarnings:\n")
		prettyErrors(buf, cr.Warnings)
	}

	if len(cr.NewObjects) != 0 {
		buf.WriteString("\nNew objects:\n")
		var refs []k8s.ObjectRef
		for _, o := range cr.NewObjects {
			refs = append(refs, o.Ref)
		}
		prettyObjectRefs(buf, refs)
	}
	if len(cr.ChangedObjects) != 0 {
		buf.WriteString("\nChanged objects:\n")
		var refs []k8s.ObjectRef
		for _, co := range cr.ChangedObjects {
			refs = append(refs, co.Ref)
		}
		prettyObjectRefs(buf, refs)

		buf.WriteString("\n")
		for _, co := range cr.ChangedObjects {
			prettyChanges(buf, co.Ref, co.Changes)
		}
	}

	if len(cr.DeletedObjects) != 0 {
		buf.WriteString("\nDeleted objects:\n")
		prettyObjectRefs(buf, cr.DeletedObjects)
	}

	if len(cr.HookObjects) != 0 {
		buf.WriteString("\nApplied hooks:\n")
		var refs []k8s.ObjectRef
		for _, o := range cr.HookObjects {
			refs = append(refs, o.Ref)
		}
		prettyObjectRefs(buf, refs)
	}
	if len(cr.OrphanObjects) != 0 {
		buf.WriteString("\nOrphan objects:\n")
		prettyObjectRefs(buf, cr.OrphanObjects)
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

func prettyErrors(buf io.StringWriter, errors []types.DeploymentError) {
	for _, e := range errors {
		_, _ = buf.WriteString(fmt.Sprintf("  %s: %s\n", e.Ref.String(), e.Error))
	}
}

func prettyChanges(buf io.StringWriter, ref k8s.ObjectRef, changes []types.Change) {
	_, _ = buf.WriteString(fmt.Sprintf("Diff for object %s\n", ref.String()))

	var t utils.PrettyTable
	t.AddRow("Path", "Diff")

	for _, c := range changes {
		t.AddRow(c.JsonPath, c.UnifiedDiff)
	}
	s := t.Render([]int{60})
	_, _ = buf.WriteString(s)
}

func formatCommandResultYaml(cr *types.CommandResult) (string, error) {
	b, err := yaml.WriteYamlString(cr)
	if err != nil {
		return "", err
	}
	return b, nil
}

func formatCommandResult(cr *types.CommandResult, format string) (string, error) {
	switch format {
	case "text":
		return formatCommandResultText(cr), nil
	case "yaml":
		return formatCommandResultYaml(cr)
	default:
		return "", fmt.Errorf("invalid format: %s", format)
	}
}

func prettyValidationResults(buf io.StringWriter, results []types.ValidateResultEntry) {
	var t utils.PrettyTable
	t.AddRow("Object", "Message")

	for _, e := range results {
		t.AddRow(e.Ref.String(), e.Message)
	}
	s := t.Render([]int{60})
	_, _ = buf.WriteString(s)
}

func formatValidateResultText(vr *types.ValidateResult) string {
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
	return buf.String()
}

func formatValidateResultYaml(vr *types.ValidateResult) (string, error) {
	b, err := yaml.WriteYamlString(vr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func formatValidateResult(vr *types.ValidateResult, format string) (string, error) {
	switch format {
	case "text":
		return formatValidateResultText(vr), nil
	case "yaml":
		return formatValidateResultYaml(vr)
	default:
		return "", fmt.Errorf("invalid validation result format: %s", format)
	}
}

func outputHelper(output []string, cb func(format string) (string, error)) error {
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

		err = outputResult(path, r)
		if err != nil {
			return err
		}
	}
	return nil
}

func outputCommandResult(output []string, cr *types.CommandResult) error {
	return outputHelper(output, func(format string) (string, error) {
		return formatCommandResult(cr, format)
	})
}

func outputValidateResult(output []string, vr *types.ValidateResult) error {
	return outputHelper(output, func(format string) (string, error) {
		return formatValidateResult(vr, format)
	})
}

func outputYamlResult(output []string, result interface{}, multiDoc bool) error {
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
		err := outputResult(&path, s)
		if err != nil {
			return err
		}
	}
	return nil
}

func outputResult(f *string, result string) error {
	w := os.Stdout
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
