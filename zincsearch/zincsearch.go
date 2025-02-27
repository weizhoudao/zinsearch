package zincsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"zincsearch/lib"
	"net/http"
)

type FieldSetting struct{
	Type string `json:"type,omitempty"`
	Index bool `json:"index",omitempty`
	Sortable bool `json:"sortable:omitempty"`
	Store bool `json:"store"`
}

type Document struct {
	Title string `json:"title"`
	Description string `json:"description"`
	ChatID string `json:"chat_id"`
	UserCount int `json:"user_count"`
	JsName string `json:"js_name"`
	// js_type: qm hs
	JsType string `json:"js_type"`
	Location string `json:"location"`
	Tags string `json:"tags"`
}

type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// 索引配置结构体
type IndexSettings struct {
	Name             string                             `json:"name"`
	NumberOfShards   int                    `json:"shard_num"`
	Storagetype      string                      `json:"storage_type"`
	Mappings         map[string]interface{} `json:"mappings"`
}

type IndexSettingsList struct {
	List []IndexSettings `json:"list"`
}

// 搜索请求结构体
type SearchRequest struct {
	SearchType string                 `json:"search_type"`
	Query      map[string]interface{} `json:"query"`
	From       int                    `json:"from"`
	MaxResults int                    `json:"max_results"`
	SortFields []string               `json:"sort_fields"`
}

// 搜索响应结构体
type SearchResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID     string                 `json:"_id"`
			Source map[string]interface{} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// 错误响应结构体
type ErrorResponse struct {
	Error string `json:"error"`
}

func NewClient(baseURL, username, password string) *Client {
	return &Client{
		baseURL:    baseURL,
		username:   username,
		password:   password,
		httpClient: &http.Client{},
	}
}

// 创建索引
func (c *Client) CreateIndex(settings *IndexSettings) error {
	url := fmt.Sprintf("%s/api/index", c.baseURL)
	return c.doRequest("POST", url, settings, nil)
}

// 删除索引
func (c *Client) DeleteIndex(indexName string) error {
	url := fmt.Sprintf("%s/api/index/%s", c.baseURL, indexName)
	return c.doRequest("DELETE", url, nil, nil)
}

// 获取索引列表
func (c *Client) ListIndexes() ([]string, error) {
	url := fmt.Sprintf("%s/api/index", c.baseURL)
	var indexes IndexSettingsList

	err := c.doRequest("GET", url, nil, &indexes)
	if err != nil {
		return nil, err
	}
	lib.XLogInfo(indexes)
	result := make([]string, len(indexes.List))
	for i, index := range indexes.List {
		result[i] = index.Name
	}
	return result, nil
}

// 插入文档
func (c *Client) InsertDocument(indexName string, document interface{}) error {
	url := fmt.Sprintf("%s/api/%s/_doc", c.baseURL, indexName)
	return c.doRequest("POST", url, document, nil)
}

// 更新文档
func (c *Client) UpdateDocument(indexName, docID string, document interface{}) error {
	url := fmt.Sprintf("%s/api/%s/_doc/%s", c.baseURL, indexName, docID)
	return c.doRequest("PUT", url, document, nil)
}

// 删除文档
func (c *Client) DeleteDocument(indexName, docID string) error {
	url := fmt.Sprintf("%s/api/%s/_doc/%s", c.baseURL, indexName, docID)
	return c.doRequest("DELETE", url, nil, nil)
}

// 搜索文档
func (c *Client) Search(indexName string, req *SearchRequest) (*SearchResponse, error) {
	url := fmt.Sprintf("%s/api/%s/_search", c.baseURL, indexName)
	var response SearchResponse
	err := c.doRequest("POST", url, req, &response)
	return &response, err
}

// 通用请求处理
func (c *Client) doRequest(method, url string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return err
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("zincsearch error (%d): %s", resp.StatusCode, errResp.Error)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}
