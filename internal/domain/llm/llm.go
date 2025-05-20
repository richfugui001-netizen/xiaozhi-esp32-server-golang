package llm

import (
	"bytes"
	"context"
	"strings"
	"time"
	"unicode"
	"xiaozhi-esp32-server-golang/internal/domain/llm/common"
	log "xiaozhi-esp32-server-golang/logger"
)

// 句子结束的标点符号
var sentenceEndPunctuation = []rune{'.', '。', '!', '！', '?', '？', '\n'}

// 句子暂停的标点符号（可以作为长句子的断句点）
var sentencePausePunctuation = []rune{',', '，', ';', '；', ':', '：'}

// 判断一个字符是否为句子结束的标点符号
func isSentenceEndPunctuation(r rune) bool {
	for _, p := range sentenceEndPunctuation {
		if r == p {
			return true
		}
	}
	return false
}

// 判断一个字符是否为句子暂停的标点符号
func isSentencePausePunctuation(r rune) bool {
	for _, p := range sentencePausePunctuation {
		if r == p {
			return true
		}
	}
	return false
}

// HandleLLMWithContext 使用上下文控制来处理LLM响应
func HandleLLMWithContext(ctx context.Context, llmProvider LLMProvider, dialogue []interface{}, sessionID string) (chan common.LLMResponseStruct, error) {
	// 使用支持上下文的响应方法
	llmResponse := llmProvider.ResponseWithContext(ctx, sessionID, dialogue)

	sentenceChannel := make(chan common.LLMResponseStruct, 2)

	startTs := time.Now().UnixMilli()
	var firstFrame bool

	fullText := ""
	var buffer bytes.Buffer // 用于累积接收到的内容
	isFirst := true
	go func() {
		defer func() {
			log.Debugf("full Response: %s", fullText)
			close(sentenceChannel)
		}()
		for {
			select {
			case response, ok := <-llmResponse:
				if !ok {
					// llmResponse通道已关闭，处理剩余内容
					remaining := buffer.String()
					log.Infof("处理剩余内容: %s", remaining)
					fullText += remaining
					sentenceChannel <- common.LLMResponseStruct{
						Text:  remaining,
						IsEnd: true,
					}
					return
				}
				fullText += response

				// 将响应片段添加到累积缓冲区
				buffer.WriteString(response)

				if containsSentenceSeparator(response, isFirst) {
					// 检查缓冲区中是否包含完整的句子
					sentences, remaining := extractSmartSentences(buffer.String(), 5, 100, isFirst)

					// 如果有完整的句子，处理它们
					if len(sentences) > 0 {
						for _, sentence := range sentences {
							if sentence != "" {
								if !firstFrame {
									firstFrame = true
									log.Infof("llm首句耗时: %d ms", time.Now().UnixMilli()-startTs)
								}
								log.Infof("处理完整句子: %s", sentence)
								// 发送完整句子给客户端
								sentenceChannel <- common.LLMResponseStruct{
									Text:    sentence,
									IsStart: isFirst,
									IsEnd:   false,
								}
								if isFirst {
									isFirst = false
								}
							}
						}
					}

					// 更新缓冲区为剩余内容
					buffer.Reset()
					buffer.WriteString(remaining)
				}

			case <-ctx.Done():
				// 上下文已取消，立即停止处理并返回
				log.Infof("上下文已取消，停止LLM响应处理: %v", ctx.Err())
				return
			}
		}
	}()
	return sentenceChannel, nil
}

// 判断字符串是否为数字加点号格式（如"1."、"2."等）
func isNumberWithDot(s string) bool {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < 2 || trimmed[len(trimmed)-1] != '.' {
		return false
	}

	for i := 0; i < len(trimmed)-1; i++ {
		if !unicode.IsDigit(rune(trimmed[i])) {
			return false
		}
	}
	return true
}

// 从文本中提取完整的句子
// 返回完整句子的切片和剩余的未完成内容
func extractCompleteSentences(text string) ([]string, string) {
	if text == "" {
		return []string{}, ""
	}

	var sentences []string
	var currentSentence bytes.Buffer

	runes := []rune(text)
	lastIndex := len(runes) - 1

	for i, r := range runes {
		currentSentence.WriteRune(r)

		// 判断句子是否结束
		if isSentenceEndPunctuation(r) {
			// 如果是句子结束标点
			sentence := strings.TrimSpace(currentSentence.String())
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			currentSentence.Reset()
		} else if i == lastIndex {
			// 如果是最后一个字符但不是句子结束标点，保留在remaining中
			break
		}
	}

	// 当前未完成的句子作为remaining返回
	remaining := currentSentence.String()
	return sentences, strings.TrimSpace(remaining)
}
