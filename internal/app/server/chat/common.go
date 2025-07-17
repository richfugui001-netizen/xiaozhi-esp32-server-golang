package chat

func (s *ChatSession) StopSpeaking(isSendTtsStop bool) {
	if isSendTtsStop {
		s.serverTransport.SendTtsStop()
	}

	s.clientState.CancelSessionCtx()
	s.llmManager.ClearLLMResponseQueue()
	s.ClearChatTextQueue()
}

func (s *ChatSession) MqttClose() {
	s.serverTransport.SendMqttGoodbye()
}
