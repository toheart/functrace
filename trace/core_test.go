package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsStructMethod(t *testing.T) {
	// 创建测试实例
	traceInstance := NewTraceInstance()

	// 定义测试用例
	testCases := []struct {
		name           string
		fullName       string
		expectedResult int
		description    string
	}{
		{
			name:           "normalFunction",
			fullName:       "k8s.io/client-go/rest.RESTClientForConfigAndClient",
			expectedResult: MethodTypeNormal,
			description:    "普通包级函数",
		},
		{
			name:           "valueReceiverFunction",
			fullName:       "github.com/example/package.Type.Method",
			expectedResult: MethodTypeValue,
			description:    "类型的值接收者方法",
		},
		{
			name:           "multiPackageValueReceiverFunction",
			fullName:       "github.com/example/package/subpackage.Type.Method",
			expectedResult: MethodTypeValue,
			description:    "multiPackageValueReceiverFunction",
		},
		{
			name:           "pointerReceiverFunction",
			fullName:       "k8s.io/client-go/rest.(*Request).newStreamWatcher",
			expectedResult: MethodTypePointer,
			description:    "类型的指针接收者方法",
		},
		{
			name:           "multiPackagePointerReceiverFunction",
			fullName:       "github.com/example/package/subpackage.(*Type).Method",
			expectedResult: MethodTypePointer,
			description:    "多段包名中的指针接收者方法",
		},
		{
			name:           "unknownFunctionNameFormat",
			fullName:       "SingleWordFunction",
			expectedResult: MethodTypeUnknown,
			description:    "未包含点号的函数名",
		},
		{
			name:           "specialCaseWithParenthesesButNotPointerReceiver",
			fullName:       "github.com/example/package.(Type).Method",
			expectedResult: MethodTypeValue,
			description:    "包含括号但不是指针接收者的方法",
		},
		{
			name:           "nestedTypeMethod",
			fullName:       "github.com/example/package.Outer.Inner.Method",
			expectedResult: MethodTypeValue,
			description:    "嵌套类型的方法",
		},
		{
			name:           "nestedPointerTypeMethod",
			fullName:       "github.com/example/package.(*Outer).(*Inner).Method",
			expectedResult: MethodTypePointer,
			description:    "nestedPointerTypeMethod",
		},
	}

	// 执行测试
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := traceInstance.isStructMethod(tc.fullName)
			assert.Equal(t, tc.expectedResult, result.Type, tc.description)
			t.Logf("result: %v", result)
		})
	}
}
