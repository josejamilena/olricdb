// Copyright 2018 Burak Sezer
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package olricdb

import (
	"encoding/gob"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/julienschmidt/httprouter"
)

func (h *httpTransport) handleExGet(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := ps.ByName("name")
	key := ps.ByName("key")
	dm := h.db.NewDMap(name)
	value, err := dm.Get(key)
	if err == ErrKeyNotFound {
		h.returnErr(w, err, http.StatusNotFound)
		return
	}
	if err != nil {
		h.returnErr(w, err, http.StatusInternalServerError)
		return
	}
	err = gob.NewEncoder(w).Encode(&value)
	if err != nil {
		h.returnErr(w, err, http.StatusInternalServerError)
		return
	}
}

func (h *httpTransport) handleExPut(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := ps.ByName("name")
	key := ps.ByName("key")
	owner, hkey, err := h.db.locateKey(name, key)
	if err != nil {
		h.returnErr(w, err, http.StatusInternalServerError)
		return
	}

	if !hostCmp(owner, h.db.this) {
		// Redirect this to the its owner.
		target := url.URL{
			Scheme: h.scheme,
			Host:   owner.String(),
			Path:   path.Join("/put/", name, printHKey(hkey)),
		}
		rtimeout := r.URL.Query().Get("t")
		if rtimeout != "" {
			q := target.Query()
			q.Set("t", rtimeout)
			target.RawQuery = q.Encode()
		}
		_, err = h.doRequest(http.MethodPost, target, r.Body)
		if err != nil {
			h.returnErr(w, err, http.StatusInternalServerError)
		}
		return
	}

	// We need to add a fallback option here to handle requests outside from Go,
	// such as a possible Python client. JSON or Msgpack can be used for encoding
	// at client side.
	var value interface{}
	err = gob.NewDecoder(r.Body).Decode(&value)
	if err != nil {
		h.returnErr(w, err, http.StatusInternalServerError)
		return
	}

	var timeout = nilTimeout
	rtimeout := r.URL.Query().Get("t")
	if rtimeout != "" {
		timeout, err = time.ParseDuration(rtimeout)
		if err != nil {
			h.returnErr(w, err, http.StatusInternalServerError)
			return
		}
	}

	// FIXME: The following call may be useless. Check it.
	registerValueType(value)
	err = h.db.putKeyVal(hkey, name, value, timeout)
	if err != nil {
		h.returnErr(w, err, http.StatusInternalServerError)
		return
	}
}

func (h *httpTransport) handleExDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := ps.ByName("name")
	key := ps.ByName("key")
	dm := h.db.NewDMap(name)
	err := dm.Delete(key)
	if err != nil {
		h.returnErr(w, err, http.StatusInternalServerError)
		return
	}
}

func (h *httpTransport) handleExLockWithTimeout(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := ps.ByName("name")
	key := ps.ByName("key")

	rtimeout := r.URL.Query().Get("t")
	timeout, err := time.ParseDuration(rtimeout)
	if err != nil {
		h.returnErr(w, err, http.StatusInternalServerError)
		return
	}

	dm := h.db.NewDMap(name)
	err = dm.LockWithTimeout(key, timeout)
	if err == ErrKeyNotFound {
		h.returnErr(w, err, http.StatusNotFound)
		return
	}
	if err != nil {
		h.returnErr(w, err, http.StatusInternalServerError)
		return
	}
}

func (h *httpTransport) handleExUnlock(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := ps.ByName("name")
	key := ps.ByName("key")

	dm := h.db.NewDMap(name)
	err := dm.Unlock(key)
	if err == ErrNoSuchLock {
		h.returnErr(w, err, http.StatusNotFound)
		return
	}
	if err != nil {
		h.returnErr(w, err, http.StatusInternalServerError)
		return
	}
}

func (h *httpTransport) handleExDestroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := ps.ByName("name")
	dm := h.db.NewDMap(name)
	err := dm.Destroy()
	if err != nil {
		h.returnErr(w, err, http.StatusInternalServerError)
		return
	}
}
