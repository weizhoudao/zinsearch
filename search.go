package zincsearch

import(
	"net/http"
	"encoding/json"
	"fmt"
	"io"
)

/*
{
    "search_type": "match",
    "query": {
        "term": "shell window",
        "field": "_all",
        "start_time": "2021-12-25T15:08:48.777Z",
        "end_time": "2021-12-28T16:08:48.777Z"
    },
    "sort_fields": ["-@timestamp"],
    "from": 0,
    "max_results": 20,
    "_source": [
        "Field1", "Field2" // 将其保留为空数组，则返回所有字段。
    ]
}
*/

// http请求
// method=请求方式(GET POST PUT DELETE)，url=请求地址，data请求数据
func RequestHttp(method, url, data string) (string, error) {
    payload := strings.NewReader(data)
    client := &http.Client{}
    req, err := http.NewRequest(method, url, payload)
    if err != nil {
        return "", err
    }
    req.Header.Add("Content-Type", "application/json")
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    return string(body), nil
}

func Search(offset, limit int, index, keyword string, sources []string)(res string, err error){
	url := fmt.Sprintf("", "http://localhost:4080/api/", index, "/_search")
}
