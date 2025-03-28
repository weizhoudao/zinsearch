package model

import (
	"encoding/json"
	"bytes"
	"zincsearch/lib"
	"reflect"
	"net/http"
	"io/ioutil"
	"errors"
	"time"
)

type KeyStatus struct{
	Key string
	IsBlock bool
	BlockTo int64
}

type TBot struct {
	BotKey string
	BakKey string
	UseBakKey bool
	BakKeys []KeyStatus
	ShutdownChannel chan interface{}
}

func (bot *TBot)Request(method, param string)(string, error){
	url := "https://api.telegram.org/" + bot.BotKey + "/" + method
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(param)))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	client := &http.Client{}
	rsp, err := client.Do(req)
	if err != nil {
		lib.XLogErr("client.Do", req, err)
		return "", err
	}
	defer rsp.Body.Close()
	body, err := ioutil.ReadAll(rsp.Body)
	return string(body), err
}

func (bot *TBot)RequestV2(key, method, param string)(string, error){
	url := "https://api.telegram.org/" + key + "/" + method
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(param)))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	client := &http.Client{}
	rsp, err := client.Do(req)
	if err != nil {
		lib.XLogErr("client.Do", req, err)
		return "", err
	}
	defer rsp.Body.Close()
	body, err := ioutil.ReadAll(rsp.Body)
	return string(body), err
}

func (bot *TBot)DoCall(key, method, param string)(APIResponse, error){
	var api_res APIResponse
	rsp, err := bot.RequestV2(key, method, param)
	if err != nil {
		lib.XLogErr("RequestV2", method, err, param)
		return api_res, err
	}
	err = json.Unmarshal([]byte(rsp), &api_res)
	if err != nil {
		lib.XLogErr("Unmarshal", rsp, err)
		return api_res, err
	}
	if !api_res.Ok {
		lib.XLogErr("not ok", string(param), api_res)
		return api_res, errors.New(api_res.Description)
	}
	return api_res, nil
}

func (bot *TBot)CallV2(config interface{})error{
	obj_type := reflect.TypeOf(config).Elem().Name()
	method := obj_type[: len(obj_type) - 6]
	param, err := json.Marshal(config)
	if err != nil {
		lib.XLogErr("json.Marshal", config)
		return err
	}
	var api_res APIResponse
	if bot.UseBakKey{
		hit := false
		for i, item := range bot.BakKeys{
			if item.IsBlock{
				if item.BlockTo <= time.Now().Unix() {
					continue
				}else{
					bot.BakKeys[i].IsBlock = false
				}
			}
			hit = true
			api_res, err = bot.DoCall(item.Key, method, string(param))
			if err != nil {
				if api_res.ErrorCode == 429{
					lib.XLogErr("change key and continue")
					bot.BakKeys[i].IsBlock = true
					bot.BakKeys[i].BlockTo = time.Now().Unix() + 60
					continue
				}
				return err
			}
			break
		}
		if hit == false{
			return errors.New("all key block")
		}
	}else{
		api_res, err = bot.DoCall(bot.BotKey, method, string(param))
		if err != nil{
			return err
		}
	}
	obj_val := reflect.ValueOf(config)
	res_value := obj_val.Elem().FieldByName("Response")
	tmp_obj := reflect.New(reflect.TypeOf(res_value.Interface()))
	err = json.Unmarshal(api_res.Result, tmp_obj.Interface())
	if err != nil {
		lib.XLogErr("json.Unmarshal", api_res.Result, err)
		return err
	}
	res_value.Set(tmp_obj.Elem())
	return nil
}

func (bot *TBot)Call(config interface{})error{
	obj_type := reflect.TypeOf(config).Elem().Name()
	method := obj_type[: len(obj_type) - 6]

	param, err := json.Marshal(config)
	if err != nil {
		lib.XLogErr("json.Marshal", config)
		return err
	}

	rsp, err := bot.Request(method, string(param))

	if err != nil {
		lib.XLogErr("bot.Request", method, param)
		return err
	}

	var api_res APIResponse
	err = json.Unmarshal([]byte(rsp), &api_res)
	if err != nil {
		lib.XLogErr("json.Unmarshal", rsp, err)
		return err
	}
	if !api_res.Ok {
		lib.XLogErr("not ok", string(param), api_res)
		return errors.New(api_res.Description)
	}

	obj_val := reflect.ValueOf(config)
	res_value := obj_val.Elem().FieldByName("Response")
	tmp_obj := reflect.New(reflect.TypeOf(res_value.Interface()))
	err = json.Unmarshal(api_res.Result, tmp_obj.Interface())
	if err != nil {
		lib.XLogErr("json.Unmarshal", api_res.Result, err)
		return err
	}
	res_value.Set(tmp_obj.Elem())
	return nil
}

func (bot *TBot)GetUpdates(config *UpdateConfig)error{
	data, err := json.Marshal(config)
	if err != nil {
		lib.XLogErr("json.Marshal", config, err)
		return err
	}
	rsp, err := bot.Request("getupdates", string(data))
	if err != nil {
		lib.XLogErr("Request", config, err)
		return err
	}
	lib.XLogInfo(rsp)
	var api_res APIResponse
	err = json.Unmarshal([]byte(rsp), &api_res)
	if err != nil {
		lib.XLogErr("json.Unmarshal", rsp, err)
		return err
	}
	if !api_res.Ok {
		lib.XLogErr("not ok", api_res)
		return errors.New(api_res.Description)
	}
	err = json.Unmarshal(api_res.Result, &config.Response)
	if err != nil {
		lib.XLogErr("json.Unmarshal", api_res.Result, err)
		return err
	}
	return nil
}

func (bot *TBot)GetUpdateChan(config *UpdateConfig)<-chan Update{
	ch := make(chan Update, 20)
	go func(){
		for {
			select {
			case <-bot.ShutdownChannel:
				close(ch)
				return
			default:
			}
			config.Response = nil
			err := bot.GetUpdates(config)
			if err != nil {
				lib.XLogErr("bot.GetUpdates", *config, err)
				continue
			}
			for _, val := range config.Response {
				if val.UpdateID >= config.Offset {
					config.Offset = val.UpdateID + 1
					ch <- val
				}
			}
		}
	}()
	return ch
}
