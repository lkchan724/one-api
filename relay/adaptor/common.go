package adaptor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/client"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/relay/meta"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
)

func SetupCommonRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) {
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	req.Header.Set("Accept", c.Request.Header.Get("Accept"))
	if meta.IsStream && c.Request.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "text/event-stream")
	}
}
func removeNameFromSystemRoles(reader io.Reader) (io.Reader, error) {
	// 先将io.Reader中的JSON数据解析为map[string]interface{}
	var data map[string]interface{}
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&data)
	if err!= nil {
		return nil, err
	}

	// 假设JSON中有"messages"数组，且数组元素里包含role字段，进行相应处理
	if messages, ok := data["messages"].([]interface{}); ok {
		for _, msg := range messages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				if role, ok := msgMap["role"].(string); ok && role == "system" {
					// 如果role是system，删除name字段
					delete(msgMap, "name")
				}
			}
		}
	}

	// 将修改后的数据重新序列化为JSON格式的字节切片
	newJsonBytes, err := json.Marshal(data)
	if err!= nil {
		return nil, err
	}

	// 将字节切片包装成io.Reader返回
	return bytes.NewReader(newJsonBytes), nil
}

func DoRequestHelper(a Adaptor, c *gin.Context, meta *meta.Meta, requestBody io.Reader) (*http.Response, error) {
	fullRequestURL, err := a.GetRequestURL(meta)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	isXAi := strings.Contains(fullRequestURL, "api.x.ai")
	logger.SysLogf("fullRequestURL="+fullRequestURL)
	if isXAi {
		newBody, err := removeNameFromSystemRoles(requestBody)
		if err != nil {
			logger.SysLogf("Request replace error ", err)
		} else {
			logger.SysLogf("Request replace success ", err)
			requestBody = newBody
		}
	}
	req, err := http.NewRequest(c.Request.Method, fullRequestURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	err = a.SetupRequestHeader(c, req, meta)
	// 打印请求头
	dumpRequest, err := httputil.DumpRequest(req, true)
	if err != nil {
		logger.SysLogf("Error dumping request:%s", err)
	} else {
		logger.SysLogf("Request Headers:")
		logger.SysLogf(string(dumpRequest))
	}
	if err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	resp, err := DoRequest(c, req)

	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}

	return resp, nil
}

func DoRequest(c *gin.Context, req *http.Request) (*http.Response, error) {
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("resp is nil")
	}
	_ = req.Body.Close()
	_ = c.Request.Body.Close()
	return resp, nil
}
