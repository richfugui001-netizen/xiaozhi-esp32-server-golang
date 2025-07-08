package common

import (
	. "xiaozhi-esp32-server-golang/internal/data/client"
)

func StopSpeaking(clientState *ClientState, isSendTtsStop bool) {
	clientState.CancelSessionCtx()
	if isSendTtsStop {
		SendTtsStop(clientState)
	}
}
