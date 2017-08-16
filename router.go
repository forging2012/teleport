// Copyright 2015-2017 HenryLee. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package teleport

import (
	"path"
	"reflect"

	"github.com/henrylee2cn/goutil"
	"github.com/henrylee2cn/goutil/errors"
)

type (
	// Router
	Router interface {
	}
	// ApiMap api map, that is, the router
	ApiMap struct {
		apimap map[string]*ApiType
		// only for register router
		pathPrefix string
		plugins    []Plugin
	}
)

// Group add api group.
func (m *ApiMap) Group(pathPrefix string, plugins ...Plugin) *ApiMap {
	ps := make([]Plugin, len(m.plugins)+len(plugins))
	copy(ps, m.plugins)
	copy(ps[len(m.plugins):], plugins)
	return &ApiMap{
		apimap:     m.apimap,
		pathPrefix: path.Join(m.pathPrefix, pathPrefix),
		plugins:    ps,
	}
}

func (m *ApiMap) Reg(pathPrefix string, ctrlStruct Context, plugin ...Plugin) error {
	apiTyps, err := parseApis(
		path.Join(m.pathPrefix, pathPrefix),
		ctrlStruct,
		&pluginContainer{append(m.plugins, plugin...)},
	)
	if err != nil {
		return err
	}
	for _, apiType := range apiTyps {
		name := path.Join(pathPrefix, apiType.name)
		if _, ok := m.apimap[name]; ok {
			return errors.New("There is a route conflict: " + name)
		}
		m.apimap[name] = apiType
	}
	return nil
}

// var contextType = reflect.TypeOf(Context(nil))

// Precompute the reflect type for error. Can't use error directly
// because Typeof takes an empty interface value. This is annoying.
var typeOfError = reflect.TypeOf((*Error)(nil)).Elem()

func parseApis(pathPrefix string, ctrlStruct Context, pluginContainer PluginContainer) ([]*ApiType, error) {
	var (
		ctype    = reflect.TypeOf(ctrlStruct)
		apiTypes = make([]*ApiType, 0, 1)
	)
	if pluginContainer == nil {
		pluginContainer = NewPluginContainer()
	}
	for m := 0; m < ctype.NumMethod(); m++ {
		method := ctype.Method(m)
		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs two ins: receiver, *args.
		if mtype.NumIn() != 2 {
			return nil, errors.Errorf("Handler: %s.%s needs one in argument, but have %d", ctype.String(), mname, mtype.NumIn())
		}
		// Receiver need be a struct pointer.
		structType := mtype.In(0)
		if structType.Kind() != reflect.Ptr || structType.Elem().Kind() != reflect.Struct {
			return nil, errors.Errorf("Handler: %s.%s receiver need be a struct pointer: %s", ctype.String(), mname, structType)
		}
		// First arg need be exported or builtin, and need be a pointer.
		argType := mtype.In(1)
		if !goutil.IsExportedOrBuiltinType(argType) {
			return nil, errors.Errorf("Handler: %s.%s args type not exported: %s", ctype.String(), mname, argType)
		}
		if argType.Kind() != reflect.Ptr {
			return nil, errors.Errorf("Handler: %s.%s args type need be a pointer: %s", ctype.String(), mname, argType)
		}
		// Method needs two outs: reply error.
		if mtype.NumOut() != 2 {
			return nil, errors.Errorf("Handler: %s.%s needs two out arguments, but have %d", ctype.String(), mname, mtype.NumOut())
		}
		// Reply type must be exported.
		replyType := mtype.Out(0)
		if !goutil.IsExportedOrBuiltinType(replyType) {
			return nil, errors.Errorf("Handler: %s.%s first reply type not exported: %s", ctype.String(), mname, replyType)
		}
		// The return type of the method must be Error.
		if returnType := mtype.Out(1); returnType != typeOfError {
			return nil, errors.Errorf("Handler: %s.%s second reply type %s not *Error", ctype.String(), mname, returnType)
		}

		apiTypes = append(apiTypes, &ApiType{
			name:            goutil.CamelString(ctype.Name() + "/" + mname),
			originStruct:    ctype,
			method:          method,
			arg:             argType,
			reply:           replyType,
			pluginContainer: pluginContainer,
		})
	}
	return apiTypes, nil
}

func (m *ApiMap) get(uriPath string) (*ApiType, bool) {
	apiType, ok := m.apimap[uriPath]
	return apiType, ok
}
