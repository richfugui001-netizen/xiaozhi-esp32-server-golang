package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ToolName string

const (
	STREAM_DONE_FLAG = "[DONE]"

	MUSIC_PLAYER   ToolName = "musicPlayer"
	VOLUME_CONTROL ToolName = "set_volume"
)

func NewMCPServer() *server.MCPServer {
	mcpServer := server.NewMCPServer(
		"example-servers/everything",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	mcpServer.AddTool(mcp.NewTool(string(MUSIC_PLAYER),
		mcp.WithDescription("音乐播放器 - 搜索和播放本地音乐文件"),
		mcp.WithString("query",
			mcp.Description("搜索关键词或文件名 "),
		),
	), handleMusicPlayerTool)

	mcpServer.AddTool(mcp.NewTool(string(VOLUME_CONTROL),
		mcp.WithDescription("调整音量大小, 当用户想调整音量时使用此工具"),
		mcp.WithNumber("volume",
			mcp.Description("音量大小，范围1-100"),
			mcp.Required(),
		),
	), handleVolumeControlTool)

	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"audio://music/{musicUrl}?start={start}&end={end}",
			"audio resource",
		),
		handleAudioResourceTemplate,
	)

	return mcpServer
}

func handleAudioResourceTemplate(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	log.Printf("request.params: %+v\n", request.Params.Arguments)

	var base64MusicUrl string
	if url, ok := request.Params.Arguments["musicUrl"]; ok {
		if realUrlList, ok := url.([]string); ok {
			base64MusicUrl = realUrlList[0]
		}
	}
	realMusicUrl, err := base64.StdEncoding.DecodeString(base64MusicUrl)
	if err != nil {
		log.Printf("base64 decode error: %+v\n", err)
		return nil, fmt.Errorf("base64 decode error: %+v", err)
	}

	var start, end int
	if strStart, ok := request.Params.Arguments["start"]; ok {
		if istart, ok := strStart.([]string); ok {
			start, _ = strconv.Atoi(istart[0])
		}
	}

	if strEnd, ok := request.Params.Arguments["end"]; ok {
		if iEnd, ok := strEnd.([]string); ok {
			end, _ = strconv.Atoi(iEnd[0])
		}
	}

	log.Printf("start: %d, end: %d\n", start, end)

	audioData, err := GetMusicDataByUrl(string(realMusicUrl), start, end)
	if err != nil {
		log.Printf("GetMusicDataByUrl, musicUrl: %s, error: %+v", string(realMusicUrl), err)
		return nil, err
	}

	log.Printf("orig audioData: %d\n", len(audioData))

	if len(audioData) == 0 {
		audioData = []byte(STREAM_DONE_FLAG)
	}

	retAudioData := base64.StdEncoding.EncodeToString(audioData)

	return []mcp.ResourceContents{
		mcp.BlobResourceContents{URI: request.Params.URI, MIMEType: "audio/mpeg", Blob: retAudioData},
	}, nil
}

func handleMusicPlayerTool(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	query := request.GetString("query", "")
	return handlePlayMusic(query)
}

func handleVolumeControlTool(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()
	volumeFloat, ok := arguments["volume"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid volume argument")
	}

	volume := int(volumeFloat)

	log.Printf("set volume value: %d\n", volume)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: "执行成功",
			},
		},
	}, nil
}

// 搜索音乐文件
func handleSearchMusic(files []string, query string) (*mcp.CallToolResult, error) {
	if query == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}

	var matchedFiles []string
	queryLower := strings.ToLower(query)

	for _, file := range files {
		if strings.Contains(strings.ToLower(file), queryLower) {
			matchedFiles = append(matchedFiles, file)
		}
	}

	if len(matchedFiles) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("🔍 没有找到包含 \"%s\" 的音乐文件", query),
				},
			},
		}, nil
	}

	searchResult := fmt.Sprintf("🔍 搜索结果 (关键词: %s, 找到%d首):\n\n", query, len(matchedFiles))
	for i, file := range matchedFiles {
		info, err := os.Stat(file)
		var sizeInfo string
		if err == nil {
			sizeInfo = fmt.Sprintf(" (%.2f MB)", float64(info.Size())/1024/1024)
		}
		searchResult += fmt.Sprintf("%d. 🎶 %s%s\n", i+1, file, sizeInfo)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: searchResult,
			},
		},
	}, nil
}

// 播放音乐文件
func handlePlayMusic(musicName string) (*mcp.CallToolResult, error) {
	realMusicName, musicUrl, err := GetMusicUrlByName(musicName)
	if err != nil {
		return nil, fmt.Errorf("播放音乐文件失败: %v", err)
	}

	base64MusicUrl := base64.StdEncoding.EncodeToString([]byte(musicUrl))

	log.Printf("realMusicName: %s, musicUrl: %s\n", realMusicName, musicUrl)
	resourceLink := fmt.Sprintf("audio://music/%s", base64MusicUrl)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewResourceLink(resourceLink, realMusicName, realMusicName, "audio/mpeg"),
		},
	}, nil
}

func main() {
	var transport string
	flag.StringVar(&transport, "t", "stdio", "Transport type (stdio or http)")
	flag.StringVar(&transport, "transport", "stdio", "Transport type (stdio or http)")
	flag.Parse()

	mcpServer := NewMCPServer()

	// Only check for "http" since stdio is the default
	if transport == "http" {
		httpServer := server.NewStreamableHTTPServer(mcpServer)
		log.Printf("HTTP server listening on :3001/mcp")
		if err := httpServer.Start(":3001"); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		if err := server.ServeStdio(mcpServer); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}
}

// 音乐搜索API响应结构
type MusicSearchResponse struct {
	Data  []MusicItem `json:"data"`
	Code  int         `json:"code"`
	Error string      `json:"error"`
}

type MusicItem struct {
	Type   string `json:"type"`
	Link   string `json:"link"`
	SongID string `json:"songid"`
	Title  string `json:"title"`
	Author string `json:"author"`
	LRC    bool   `json:"lrc"`
	URL    string `json:"url"`
	Pic    string `json:"pic"`
}

func getMusicAudioData(musicName string) ([]byte, string, string, error) {
	realMusicName, musicUrl, err := GetMusicUrlByName(musicName)
	if err != nil {
		return []byte{}, "", "", err
	}

	// 使用优化后的函数获取音频数据
	// 这里可以根据需要指定范围，比如只获取前几MB用于预览
	// 如果要获取完整文件，可以传入 -1, -1
	body, err := GetMusicDataByUrl(musicUrl, -1, -1)
	if err != nil {
		return []byte{}, "", "", fmt.Errorf("获取音频数据失败: %v", err)
	}

	// 返回第一个搜索结果的URL
	return body, realMusicName, musicUrl, nil
}

func GetMusicUrlByName(musicName string) (string, string, error) {
	client := &http.Client{}

	// 构建请求体
	data := fmt.Sprintf("input=%s&filter=name&type=migu&page=1",
		url.QueryEscape(musicName))

	req, err := http.NewRequest("POST", "https://music.txqq.pro/",
		strings.NewReader(data))
	if err != nil {
		return "", "", fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头，模拟浏览器请求
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Origin", "https://music.txqq.pro")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "https://music.txqq.pro/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("sec-ch-ua", `"Not)A;Brand";v="8", "Chromium";v="138", "Google Chrome";v="138"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)

	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("API请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

	// 解析响应
	var searchResp MusicSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return "", "", fmt.Errorf("解析响应失败: %v", err)
	}

	if searchResp.Code != 200 {
		return "", "", fmt.Errorf("API返回错误: %s", searchResp.Error)
	}

	if len(searchResp.Data) == 0 {
		return "", "", fmt.Errorf("未找到音乐: %s", musicName)
	}
	musicItem := searchResp.Data[0]
	if musicItem.URL == "" {
		return "", "", fmt.Errorf("未找到音乐: %s", musicName)
	}
	return musicItem.Title, musicItem.URL, nil
}

func GetMusicDataByUrl(musicUrl string, start, end int) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", musicUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置 Range 头来请求指定范围的数据
	hasRangeHeader := false
	if start >= 0 && end > start {
		rangeHeader := fmt.Sprintf("bytes=%d-%d", start, end-1)
		req.Header.Set("Range", rangeHeader)
		hasRangeHeader = true
	} else if start >= 0 {
		// 只指定起始位置，读取到文件末尾
		rangeHeader := fmt.Sprintf("bytes=%d-", start)
		req.Header.Set("Range", rangeHeader)
		hasRangeHeader = true
	}

	// 设置其他必要的请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "keep-alive")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	// 206 表示部分内容（Range请求成功）
	// 200 表示完整内容（服务器不支持Range或没有设置Range）
	// 416 表示Range不满足（Range Not Satisfiable）
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && hasRangeHeader && start >= 0 {
		// 当遇到416错误时，先尝试从Content-Range头获取文件完整长度
		var fileSize int64 = -1

		// 解析Content-Range头，格式通常为: "bytes */1234" 或 "bytes 0-499/1234"
		contentRange := resp.Header.Get("Content-Range")
		if contentRange != "" {
			// 查找最后一个'/'后的数字，这是文件的完整大小
			if idx := strings.LastIndex(contentRange, "/"); idx != -1 {
				sizeStr := contentRange[idx+1:]
				if sizeStr != "*" {
					if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
						fileSize = size
					}
				}
			}
		}

		// 如果无法从Content-Range获取文件大小，使用HEAD请求获取
		if fileSize == -1 {
			headReq, err := http.NewRequest("HEAD", musicUrl, nil)
			if err != nil {
				return nil, fmt.Errorf("创建HEAD请求失败: %v", err)
			}

			headReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
			headReq.Header.Set("Accept", "*/*")
			headReq.Header.Set("Connection", "keep-alive")

			headResp, err := client.Do(headReq)
			if err != nil {
				return nil, fmt.Errorf("HEAD请求失败: %v", err)
			}
			headResp.Body.Close()

			if headResp.StatusCode == http.StatusOK {
				if contentLength := headResp.Header.Get("Content-Length"); contentLength != "" {
					if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
						fileSize = size
					}
				}
			}
		}

		// 如果start位置超出或等于文件大小，返回空数据
		if fileSize != -1 && int64(start) >= fileSize {
			return []byte{}, nil
		}

		// 请求从start到文件结束的数据
		req2, err := http.NewRequest("GET", musicUrl, nil)
		if err != nil {
			return nil, fmt.Errorf("创建fallback请求失败: %v", err)
		}

		// 设置Range头请求从start到文件结束的数据
		rangeHeader := fmt.Sprintf("bytes=%d-", start)
		req2.Header.Set("Range", rangeHeader)
		req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req2.Header.Set("Accept", "*/*")
		req2.Header.Set("Connection", "keep-alive")

		resp2, err := client.Do(req2)
		if err != nil {
			return nil, fmt.Errorf("fallback HTTP请求失败: %v", err)
		}
		defer resp2.Body.Close()

		// 如果fallback请求也返回416，说明start位置超出了文件范围，返回空数据
		if resp2.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			return []byte{}, nil
		}

		if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusPartialContent {
			return nil, fmt.Errorf("fallback HTTP请求失败，状态码: %d", resp2.StatusCode)
		}

		// 读取从start到文件结束的数据
		body, err := io.ReadAll(resp2.Body)
		if err != nil {
			return nil, fmt.Errorf("读取fallback响应数据失败: %v", err)
		}

		return body, nil
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("HTTP请求失败，状态码: %d", resp.StatusCode)
	}

	// 读取响应数据
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应数据失败: %v", err)
	}

	return body, nil
}
