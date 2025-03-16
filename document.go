package zincsearch

import(
	"net/http"
	"encoding/json"
	"fmt"
	"io"
)

type Document struct
{
	Name string `json:"name"`
	Location string `json:"location"`
	Tags string `json:"tags"`
	Order int `json:"order"`
	Type string `json:"type"`
}

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

func CreateOrUpdate(index, id string, doc Document)(res string, err error){
	url := = fmt.Sprintf("%v%v%v%v","http://localhost:4080/api/", index, "/_doc/", id)
	var properties []byte
	properties, err = json.Marshal(doc)
	if err != nil {
		return
	}
	res, err = RequestHttp("PUT", url, properties)
	return
}

func Delete(index, id string)(res string, err error){
	url := fmt.Sprintf("%v%v%v%v", "http://localhost:4080/api/", index, "/_doc/", id)
	var properties []byte
	res, err = RequestHttp("DELETE", url, properties)
	return
}
