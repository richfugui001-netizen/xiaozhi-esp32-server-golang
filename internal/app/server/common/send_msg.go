package common

import (
	types_audio "xiaozhi-esp32-server-golang/internal/data/audio"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
)

func SendTtsStart(clientState *ClientState) error {

	msg := ServerMessage{
		Type:      ServerMessageTypeTts,
		State:     MessageStateStart,
		SessionID: clientState.SessionID,
	}
	err := clientState.SendMsg(msg)
	if err != nil {
		return err
	}
	clientState.SetTtsStart(true)
	return nil
}

func SendTtsStop(clientState *ClientState) error {
	msg := ServerMessage{
		Type:      ServerMessageTypeTts,
		State:     MessageStateStop,
		SessionID: clientState.SessionID,
	}
	err := clientState.SendMsg(msg)
	if err != nil {
		return err
	}
	return nil
}

func SendHello(clientState *ClientState, transport string, audioFormat *types_audio.AudioFormat) error {
	msg := ServerMessage{
		Type:        MessageTypeHello,
		Text:        "欢迎使用小智服务器",
		SessionID:   clientState.SessionID,
		Transport:   transport,
		AudioFormat: audioFormat,
	}
	return clientState.SendMsg(msg)
}

func SendIot(clientState *ClientState, msg *ClientMessage) error {
	resp := ServerMessage{
		Type:      ServerMessageTypeIot,
		Text:      msg.Text,
		SessionID: clientState.SessionID,
		State:     MessageStateSuccess,
	}
	return clientState.SendMsg(resp)
}

func SendAsrResult(clientState *ClientState, text string) error {
	resp := ServerMessage{
		Type:      ServerMessageTypeStt,
		Text:      text,
		SessionID: clientState.SessionID,
	}
	return clientState.SendMsg(resp)
}

func SendSentenceStart(clientState *ClientState, text string) error {
	response := ServerMessage{
		Type:      ServerMessageTypeTts,
		State:     MessageStateSentenceStart,
		Text:      text,
		SessionID: clientState.SessionID,
	}
	err := clientState.SendMsg(response)
	if err != nil {
		return err
	}
	clientState.SetStatus(ClientStatusTTSStart)

	return nil
}

func SendSentenceEnd(clientState *ClientState, text string) error {
	response := ServerMessage{
		Type:      ServerMessageTypeTts,
		State:     MessageStateSentenceEnd,
		Text:      text,
		SessionID: clientState.SessionID,
	}
	err := clientState.SendMsg(response)
	if err != nil {
		return err
	}
	clientState.SetStatus(ClientStatusTTSStart)
	return nil
}

func SendMcpMsg(clientState *ClientState, payload []byte) error {
	response := ServerMessage{
		Type:      MessageTypeMcp,
		SessionID: clientState.SessionID,
		PayLoad:   payload,
	}
	return clientState.SendMsg(response)
}
