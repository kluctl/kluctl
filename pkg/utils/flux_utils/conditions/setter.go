package conditions

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UnknownCondition returns a condition with Status=Unknown and the given type, reason and message.
func UnknownCondition(t, reason, messageFormat string, messageArgs ...interface{}) *metav1.Condition {
	return &metav1.Condition{
		Type:    t,
		Status:  metav1.ConditionUnknown,
		Reason:  reason,
		Message: fmt.Sprintf(messageFormat, messageArgs...),
	}
}
