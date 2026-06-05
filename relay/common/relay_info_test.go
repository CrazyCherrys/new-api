package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	newapicommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayInfoGetFinalRequestRelayFormatPrefersExplicitFinal(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		RequestConversionChain:  []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToConversionChain(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToRelayFormat(t *testing.T) {
	info := &RelayInfo{
		RelayFormat: types.RelayFormatGemini,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatGemini), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatNilReceiver(t *testing.T) {
	var info *RelayInfo
	require.Equal(t, types.RelayFormat(""), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoInitResponsesUsageInfoFromRequestCapturesBuiltInTools(t *testing.T) {
	tools, err := newapicommon.Marshal([]map[string]any{
		{
			"type":                dto.BuildInToolWebSearchPreview,
			"search_context_size": "high",
		},
		{
			"type": dto.BuildInToolFileSearch,
		},
	})
	require.NoError(t, err)

	info := &RelayInfo{}
	info.InitResponsesUsageInfoFromRequest(&dto.OpenAIResponsesRequest{
		Tools: tools,
	})

	require.NotNil(t, info.ResponsesUsageInfo)
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, dto.BuildInToolWebSearchPreview)
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, dto.BuildInToolFileSearch)
	require.Equal(t, "high", info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview].SearchContextSize)
	require.Equal(t, 0, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolFileSearch].CallCount)
}

func TestCloneRequestHeadersSkipsCanvasCompatHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set(constant.HeaderCanvasChatResponsesCompat, "secret")
	context.Request = request

	headers := cloneRequestHeaders(context)

	require.Equal(t, "Bearer test-token", headers["Authorization"])
	require.NotContains(t, headers, constant.HeaderCanvasChatResponsesCompat)
}
