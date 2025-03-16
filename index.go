package zincsearch
import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"net/http"
	"io"
)
type Index struct {
	ShardNum int    //分片数
	PageNum  int    //页数
	PageSize int    //条数
	SortBy   string //排序字段
	Desc     bool   //按降序排序
	Name     string //通过名称进行模糊查询
}

// 获取完整请求路径
func GetUrl(path string) string {
    return fmt.Sprintf("%v%v", "http://localhost:4080", path)
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

// 1.插入索引数据
// 参数：indexname=索引名称，fields=索引的字段信息(key-value数组)
func (api *Index) Insert(indexname string, fields interface{}) (res string, err error) {
	var properties []byte
	properties, err = json.Marshal(fields)
	if err != nil {
		return
	}
	weburl := GetUrl("/api/index")
	data := `{
		"name": "` + indexname + `",
		"storage_type": "disk",
		"shard_num":  ` + strconv.Itoa(api.ShardNum) + `,
		"mappings": {
			"properties":` + string(properties) + `
		}
	}`
	res, err = RequestHttp("POST", weburl, data)
	return
}
// 2.更新索引数据
// 参数：indexname=索引名称，fields=索引的字段信息(key-value数组)
func (api *Index) Update(indexname string, fields interface{}) (res string, err error) {
	var properties []byte
	properties, err = json.Marshal(fields)
	if err != nil {
		return
	}
	weburl := GetUrl("/api/index")
	data := `{
		"name": "` + indexname + `",
		"storage_type": "disk",
		"shard_num": ` + strconv.Itoa(api.ShardNum) + `,
		"mappings": {
			"properties":` + string(properties) + `
		}
	}`
	res, err = RequestHttp("PUT", weburl, data)
	return
}
// 3.删除索引数据
// 参数：indexname=索引名称
func (api *Index) Del(indexname string) (res string, err error) {
	weburl := GetUrl(fmt.Sprintf("/api/index/%v", indexname))
	res, err = RequestHttp("DELETE", weburl, "")
	return
}
// 4.列出当前已经存在的索引
func (api *Index) List() (res string, err error) {
	pathurl := fmt.Sprintf("/api/index?page_num=%v&page_size=%v&sort_by=%v&desc=%v", api.PageNum, api.PageSize, api.SortBy, api.Desc)
	if api.Name != "" {
		pathurl += fmt.Sprintf("&name=%v", api.Name)
	}
	weburl := GetUrl(pathurl)
	res, err = RequestHttp("GET", weburl, "")
	return
}
// 设置分片数-并行读取的能力，默认1
func (api *Index) SetShardNum(num int) *Index {
	api.ShardNum = num
	return api
}
// 设置分页数据
func (api *Index) Page(page, pagesize int) *Index {
	api.PageNum = page
	api.PageSize = pagesize
	return api
}
// 设置排序字段，单个字段，如：name，默认name
func (api *Index) OrderField(field string) *Index {
	api.SortBy = field
	return api
}
// 是否降序排序，默认：false
func (api *Index) IsDesc() *Index {
	api.Desc = true
	return api
}
// 通过名称进行模糊查询
func (api *Index) FindName(name string) *Index {
	api.Name = name
	return api
}
