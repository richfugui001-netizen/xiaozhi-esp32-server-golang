package chat

func StopSpeaking(serverTransport *ServerTransport, isSendTtsStop bool) {
	serverTransport.clientState.CancelSessionCtx()
	if isSendTtsStop {
		serverTransport.SendTtsStop()
	}
}
