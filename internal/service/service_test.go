package service_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/client"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	upsvc "github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/upcloud-tools/upcloud-csi/internal/service"
	"github.com/upcloud-tools/upcloud-csi/internal/service/mock"
)

func TestUpCloudService_ListBlockStorage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
		{
			"storages" : {
			   "storage" : [
					{
						"access" : "private",
						"state" : "online",
						"type" : "normal",
						"uuid" : "id1",
						"zone" : "fi-hel2"
					},
					{
						"access" : "private",
						"state" : "online",
						"type" : "normal",
						"uuid" : "id2",
						"zone" : "fi-hel2"
					},
					{
						"access" : "private",
						"state" : "online",
						"type" : "backup",
						"uuid" : "id3",
						"zone" : "fi-hel2"
					},
					{
						"access" : "private",
						"state" : "online",
						"type" : "normal",
						"uuid" : "id4",
						"zone" : "fi-hel1"
					}
			   ]
			}
		 }
		`)
	}))
	defer srv.Close()
	c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
	storages, err := c.ListBlockStorage(context.Background(), "fi-hel2")
	if err != nil {
		t.Error(err)
	}
	want := []*upcloud.Storage{
		{
			State: "online",
			Type:  "normal",
			UUID:  "id1",
			Zone:  "fi-hel2",
		},
		{
			State: "online",
			Type:  "normal",
			UUID:  "id2",
			Zone:  "fi-hel2",
		},
	}
	if len(want) != len(storages) {
		t.Errorf("storages len mismatch want %d got %d", len(want), len(storages))
		return
	}
	for i, s := range storages {
		w := want[i]
		if s.State != w.State {
			t.Errorf("storages[%d] invalid state want %s got %s", i, w.State, s.State)
		}
		if s.UUID != w.UUID {
			t.Errorf("storages[%d] invalid UUID want %s got %s", i, w.UUID, s.UUID)
		}
	}
}

func TestUpCloudService_ListBlockStorageBackups(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
		{
			"storages" : {
			   "storage" : [
					{
						"access" : "private",
						"state" : "online",
						"type" : "backup",
						"uuid" : "id1",
						"zone" : "fi-hel2",
						"origin": "id3"
					},
					{
						"access" : "private",
						"state" : "online",
						"type" : "backup",
						"uuid" : "id2",
						"zone" : "fi-hel2",
						"origin" : "id1"
					},
					{
						"access" : "private",
						"state" : "maintenance",
						"type" : "backup",
						"uuid" : "id3",
						"zone" : "fi-hel2"
					},
					{
						"access" : "private",
						"state" : "online",
						"type" : "backup",
						"uuid" : "id4",
						"zone" : "fi-hel2",
						"origin" : "id1"
					}
			   ]
			}
		 }
		`)
	}))
	defer srv.Close()

	c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
	storages, err := c.ListBlockStorageBackups(context.Background(), "id1")
	assert.NoError(t, err)
	want := []*upcloud.Storage{
		{
			State:  "online",
			Type:   "normal",
			UUID:   "id2",
			Zone:   "fi-hel2",
			Origin: "id1",
		},
		{
			State:  "online",
			Type:   "normal",
			UUID:   "id4",
			Zone:   "fi-hel2",
			Origin: "id1",
		},
	}
	assert.Len(t, storages, len(want))
	for i, s := range storages {
		w := want[i]
		if s.State != w.State {
			t.Errorf("storages[%d] invalid state want %s got %s", i, w.State, s.State)
		}
		if s.UUID != w.UUID {
			t.Errorf("storages[%d] invalid UUID want %s got %s", i, w.UUID, s.UUID)
		}
	}

	storages, err = c.ListBlockStorageBackups(context.Background(), "")
	assert.NoError(t, err)
	assert.Len(t, storages, 3)
}

func TestUpCloudService_AttachDetachBlockStorage_Concurrency(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := &mock.UpCloudClient{}
	s := service.NewUpCloudService(client)
	c := 10

	var wg sync.WaitGroup
	for i := 0; i < c; i++ {
		wg.Add(1)
		// populate backend with two nodes and add 5 storages per node
		serverUUID := fmt.Sprintf("test-node-%d", i%2)
		volUUID := fmt.Sprintf("test-vol-%d", i)
		client.StoreServer(&upcloud.ServerDetails{
			Server: upcloud.Server{
				UUID:  serverUUID,
				State: upcloud.ServerStateStarted,
			},
			StorageDevices: make([]upcloud.ServerStorageDevice, 0),
		})
		go func(volUUID, serverUUID string) {
			defer wg.Done()
			t1 := time.Now()
			err := s.AttachBlockStorage(ctx, volUUID, serverUUID)
			t.Logf("attached %s to node %s in %s", volUUID, serverUUID, time.Since(t1))
			assert.NoError(t, err)
		}(volUUID, serverUUID)
	}
	wg.Wait()
	servers, err := client.GetServers(ctx)
	require.NoError(t, err)
	require.Len(t, servers.Servers, 2)
	for _, srv := range servers.Servers {
		d, err := client.GetServerDetails(ctx, &request.GetServerDetailsRequest{UUID: srv.UUID})
		if !assert.NoError(t, err) {
			continue
		}
		for _, storage := range d.StorageDevices {
			wg.Add(1)
			go func(volUUID, serverUUID string) {
				defer wg.Done()
				t1 := time.Now()
				err := s.DetachBlockStorage(ctx, volUUID, serverUUID)
				t.Logf("detached %s from node %s in %s", volUUID, serverUUID, time.Since(t1))
				assert.NoError(t, err)
			}(storage.UUID, d.UUID)
		}
	}
	wg.Wait()
}

func TestUpCloudService_GetFileStorages(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{
				"uuid": "175d681c-813a-11f1-81d2-80fa5b957a6c",
				"name": "test-file-storage",
				"zone": "de-fra1",
				"size_gib": 250
			},
			{
				"uuid": "17aaddaa-0001-0002-0003-80fa5b957a6c",
				"name": "second-file-storage",
				"zone": "de-fra1",
				"size_gib": 500
			}
		]`)
	}))
	defer srv.Close()

	c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
	storages, err := c.GetFileStorages(context.Background())
	require.NoError(t, err)
	require.Len(t, storages, 2)
	assert.Equal(t, "175d681c-813a-11f1-81d2-80fa5b957a6c", storages[0].UUID)
	assert.Equal(t, 250, storages[0].SizeGiB)
	assert.Equal(t, "de-fra1", storages[0].Zone)
}

func TestUpCloudService_GetFileStorageByUUID(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{
				"uuid": "175d681c-813a-11f1-81d2-80fa5b957a6c",
				"name": "test-file-storage",
				"zone": "de-fra1",
				"size_gib": 250
			}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		fs, err := c.GetFileStorageByUUID(context.Background(), "175d681c-813a-11f1-81d2-80fa5b957a6c")
		require.NoError(t, err)
		assert.Equal(t, "175d681c-813a-11f1-81d2-80fa5b957a6c", fs.UUID)
		assert.Equal(t, 250, fs.SizeGiB)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error": {"error_code": "NOT_FOUND", "error_status": 404}}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		_, err := c.GetFileStorageByUUID(context.Background(), "nonexistent")
		require.ErrorIs(t, err, service.ErrFileStorageNotFound)
	})
}

func TestUpCloudService_DeleteFileStorage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// DELETE /file-storage/{uuid} - delete
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
	err := c.DeleteFileStorage(context.Background(), "175d681c-813a-11f1-81d2-80fa5b957a6c")
	require.NoError(t, err)
}

func TestUpCloudService_ModifyFileStorage(t *testing.T) {
	t.Parallel()

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch callCount {
		case 1:
			// PATCH /file-storage/{uuid} - modify
			fmt.Fprint(w, `{
				"uuid": "175d681c-813a-11f1-81d2-80fa5b957a6c",
				"name": "test-file-storage",
				"zone": "de-fra1",
				"size_gib": 300
			}`)
		case 2:
			// GET /file-storage/{uuid} - wait for operational state
			fmt.Fprint(w, `{
				"uuid": "175d681c-813a-11f1-81d2-80fa5b957a6c",
				"name": "test-file-storage",
				"zone": "de-fra1",
				"size_gib": 300,
				"operational_state": "running"
			}`)
		}
	}))
	defer srv.Close()

	c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
	fs, err := c.ModifyFileStorage(context.Background(), "175d681c-813a-11f1-81d2-80fa5b957a6c", 300)
	require.NoError(t, err)
	assert.Equal(t, 300, fs.SizeGiB)
}

func TestUpCloudService_GetFileStorageByUUID_Non404Error(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error": {"error_code": "INTERNAL_ERROR", "error_status": 500}}`)
	}))
	defer srv.Close()

	c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
	_, err := c.GetFileStorageByUUID(context.Background(), "175d681c-813a-11f1-81d2-80fa5b957a6c")
	require.Error(t, err)
	require.False(t, errors.Is(err, service.ErrFileStorageNotFound), "non-404 errors should not be wrapped")
}

func TestUpCloudService_DeleteFileStorage_Errors(t *testing.T) {
	t.Parallel()

	t.Run("GetFileStorageFails", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error": {"error_code": "NOT_FOUND", "error_status": 404}}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		err := c.DeleteFileStorage(context.Background(), "nonexistent")
		require.ErrorIs(t, err, service.ErrFileStorageNotFound)
	})

	t.Run("DeleteAPIFails", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// DELETE /file-storage/{uuid} - fails
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error": {"error_code": "INTERNAL_ERROR", "error_status": 500}}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		err := c.DeleteFileStorage(context.Background(), "175d681c-813a-11f1-81d2-80fa5b957a6c")
		require.Error(t, err)
	})
}

func TestUpCloudService_ModifyFileStorage_Error(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// PATCH /file-storage/{uuid} - fails
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error": {"error_code": "INTERNAL_ERROR", "error_status": 500}}`)
	}))
	defer srv.Close()

	c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
	_, err := c.ModifyFileStorage(context.Background(), "175d681c-813a-11f1-81d2-80fa5b957a6c", 300)
	require.Error(t, err)
}

func TestUpCloudService_NewUpCloudServiceFromCredentials(t *testing.T) {
	t.Parallel()

	t.Run("Validation", func(t *testing.T) {
		t.Parallel()
		c, err := service.NewUpCloudServiceFromCredentials("", "", "")
		require.ErrorContains(t, err, "UpCloud API credentials missing")
		require.Nil(t, c)

		c, err = service.NewUpCloudServiceFromCredentials("a", "", "")
		require.ErrorContains(t, err, "UpCloud API password is missing")
		require.Nil(t, c)

		c, err = service.NewUpCloudServiceFromCredentials("", "b", "")
		require.ErrorContains(t, err, "UpCloud API username is missing")
		require.Nil(t, c)
	})

	t.Run("Basic auth", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "a", username)
			assert.Equal(t, "b", password)
			fmt.Fprint(w, `{}`)
		}))
		defer srv.Close()
		svc, err := service.NewUpCloudServiceFromCredentials("a", "b", "", client.WithBaseURL(srv.URL))
		require.NoError(t, err)
		_, err = svc.ListBlockStorage(t.Context(), "fi-hel2")
		require.NoError(t, err)
	})

	t.Run("Token auth", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _, ok := r.BasicAuth()
			assert.False(t, ok)
			authHeader := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer c", authHeader)
			fmt.Fprint(w, `{}`)
		}))
		defer srv.Close()

		svc, err := service.NewUpCloudServiceFromCredentials("a", "b", "c", client.WithBaseURL(srv.URL))
		require.NoError(t, err)
		_, err = svc.ListBlockStorage(t.Context(), "fi-hel1")
		require.NoError(t, err)

		svc, err = service.NewUpCloudServiceFromCredentials("", "", "c", client.WithBaseURL(srv.URL))
		require.NoError(t, err)
		_, err = svc.ListBlockStorage(t.Context(), "fi-hel2")
		require.NoError(t, err)
	})
}

func TestUpCloudService_GetBlockStorageByUUID(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{
				"storage": {
					"uuid": "015d681c-813a-11f1-81d2-80fa5b957a6c",
					"size": 10,
					"state": "online",
					"type": "normal",
					"zone": "fi-hel2",
					"title": "test-vol",
					"encrypted": "no"
				}
			}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		sd, err := c.GetBlockStorageByUUID(context.Background(), "015d681c-813a-11f1-81d2-80fa5b957a6c")
		require.NoError(t, err)
		assert.Equal(t, "015d681c-813a-11f1-81d2-80fa5b957a6c", sd.UUID)
		assert.Equal(t, 10, sd.Size)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error": {"error_code": "NOT_FOUND", "error_status": 404}}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		_, err := c.GetBlockStorageByUUID(context.Background(), "nonexistent")
		require.ErrorIs(t, err, service.ErrStorageNotFound)
	})

	t.Run("APIError", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error": {"error_code": "INTERNAL_ERROR", "error_status": 500}}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		_, err := c.GetBlockStorageByUUID(context.Background(), "015d681c-813a-11f1-81d2-80fa5b957a6c")
		require.Error(t, err)
	})
}

func TestUpCloudService_GetBlockStorageByName(t *testing.T) {
	t.Parallel()

	t.Run("Found", func(t *testing.T) {
		t.Parallel()
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				// GET /storage/{type} - list storages
				fmt.Fprint(w, `{
					"storages": {
						"storage": [
							{
								"uuid": "015d681c-813a-11f1-81d2-80fa5b957a6c",
								"size": 10,
								"state": "online",
								"type": "normal",
								"zone": "fi-hel2",
								"title": "test-vol"
							}
						]
					}
				}`)
				return
			}
			// GET /storage/{uuid} - storage details
			fmt.Fprint(w, `{
				"storage": {
					"uuid": "015d681c-813a-11f1-81d2-80fa5b957a6c",
					"size": 10,
					"state": "online",
					"type": "normal",
					"zone": "fi-hel2",
					"title": "test-vol",
					"encrypted": "no"
				}
			}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		volumes, err := c.GetBlockStorageByName(context.Background(), "test-vol")
		require.NoError(t, err)
		require.Len(t, volumes, 1)
		assert.Equal(t, "015d681c-813a-11f1-81d2-80fa5b957a6c", volumes[0].UUID)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"storages": {"storage": []}}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		volumes, err := c.GetBlockStorageByName(context.Background(), "nonexistent")
		require.NoError(t, err)
		assert.Empty(t, volumes)
	})
}

func TestUpCloudService_CreateBlockStorage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"storage": {
				"uuid": "015d681c-813a-11f1-81d2-80fa5b957a6c",
				"size": 10,
				"state": "online",
				"type": "normal",
				"zone": "fi-hel2",
				"title": "test-vol"
			}
		}`)
	}))
	defer srv.Close()

	c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
	sd, err := c.CreateBlockStorage(context.Background(), &request.CreateStorageRequest{
		Zone:  "fi-hel2",
		Title: "test-vol",
		Size:  10,
	})
	require.NoError(t, err)
	assert.Equal(t, "015d681c-813a-11f1-81d2-80fa5b957a6c", sd.UUID)
	assert.Equal(t, "online", sd.State)
}

func TestUpCloudService_DeleteBlockStorage(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		err := c.DeleteBlockStorage(context.Background(), "015d681c-813a-11f1-81d2-80fa5b957a6c")
		require.NoError(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error": {"error_code": "NOT_FOUND", "error_status": 404}}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		err := c.DeleteBlockStorage(context.Background(), "nonexistent")
		require.ErrorIs(t, err, service.ErrStorageNotFound)
	})
}

func TestUpCloudService_RequireBlockStorageOnline(t *testing.T) {
	t.Parallel()

	t.Run("AlreadyOnline", func(t *testing.T) {
		t.Parallel()
		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL("http://localhost:1"))))
		err := c.RequireBlockStorageOnline(context.Background(), &upcloud.Storage{State: upcloud.StorageStateOnline})
		require.NoError(t, err)
	})

	t.Run("WaitsForOnline", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{
				"storage": {
					"uuid": "015d681c-813a-11f1-81d2-80fa5b957a6c",
					"size": 10,
					"state": "online",
					"type": "normal",
					"zone": "fi-hel2",
					"title": "test-vol"
				}
			}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		err := c.RequireBlockStorageOnline(context.Background(), &upcloud.Storage{
			UUID:  "015d681c-813a-11f1-81d2-80fa5b957a6c",
			State: upcloud.StorageStateMaintenance,
		})
		require.NoError(t, err)
	})
}

func TestUpCloudService_GetServerByHostname(t *testing.T) {
	t.Parallel()

	t.Run("Found", func(t *testing.T) {
		t.Parallel()
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				// GET /server - list servers
				fmt.Fprint(w, `{
					"servers": {
						"server": [
							{
								"uuid": "server-uuid-1",
								"hostname": "test-node",
								"zone": "fi-hel2",
								"state": "started"
							}
						]
					}
				}`)
				return
			}
			// GET /server/{uuid} - server details
			fmt.Fprint(w, `{
				"server": {
					"uuid": "server-uuid-1",
					"hostname": "test-node",
					"zone": "fi-hel2",
					"state": "started"
				}
			}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		sd, err := c.GetServerByHostname(context.Background(), "test-node")
		require.NoError(t, err)
		assert.Equal(t, "server-uuid-1", sd.UUID)
		assert.Equal(t, "fi-hel2", sd.Zone)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"servers": {"server": []}}`)
		}))
		defer srv.Close()

		c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
		_, err := c.GetServerByHostname(context.Background(), "nonexistent")
		require.ErrorIs(t, err, service.ErrServerNotFound)
	})
}

func TestUpCloudService_ResizeBlockDevice(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// PATCH /storage/{uuid}
		fmt.Fprint(w, `{
			"storage": {
				"uuid": "015d681c-813a-11f1-81d2-80fa5b957a6c",
				"size": 20,
				"state": "online",
				"type": "normal",
				"zone": "fi-hel2",
				"title": "resized-vol"
			}
		}`)
	}))
	defer srv.Close()

	c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
	sd, err := c.ResizeBlockDevice(context.Background(), "015d681c-813a-11f1-81d2-80fa5b957a6c", 20)
	require.NoError(t, err)
	assert.Equal(t, 20, sd.Size)
}

func TestUpCloudService_GetBlockStorageBackupByName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"storages": {
				"storage": [
					{
						"uuid": "backup-uuid-1",
						"title": "snappy",
						"type": "backup",
						"state": "online",
						"size": 10,
						"origin": "015d681c-813a-11f1-81d2-80fa5b957a6c",
						"zone": "fi-hel2"
					}
				]
			}
		}`)
	}))
	defer srv.Close()

	c := service.NewUpCloudService(upsvc.New(client.New("", "", client.WithBaseURL(srv.URL))))
	s, err := c.GetBlockStorageBackupByName(context.Background(), "snappy")
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, "snappy", s.Title)
	assert.Equal(t, "backup-uuid-1", s.UUID)
}
