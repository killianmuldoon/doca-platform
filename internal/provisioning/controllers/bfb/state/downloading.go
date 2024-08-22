/*
Copyright 2024 NVIDIA

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package state

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	butil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/bfb/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util/future"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type bfbDownloadingState struct {
	bfb *provisioningv1.Bfb
}

func (st *bfbDownloadingState) Handle(ctx context.Context, client client.Client) (provisioningv1.BfbStatus, error) {
	logger := log.FromContext(ctx)
	state := st.bfb.Status.DeepCopy()
	bfbTaskName := cutil.GenerateBFBTaskName(*st.bfb)
	bfbFilePath := cutil.GenerateBFBFilePath(st.bfb.Spec.FileName)

	if isDeleting(st.bfb) {
		// Retrieve and call the cancel function to stop the download
		if cancelFunc, ok := butil.DownloadingTaskMap.Load(bfbTaskName + "cancel"); ok {
			cancelFunc.(context.CancelFunc)()
			butil.DownloadingTaskMap.Delete(bfbTaskName)
			butil.DownloadingTaskMap.Delete(bfbTaskName + "cancel")
		}
		state.Phase = provisioningv1.BfbDeleting
		return *state, nil
	}

	exist, err := IsBfbExist(ctx, st.bfb.Spec.FileName)
	if err != nil {
		state.Phase = provisioningv1.BfbError
		return *state, err
	} else if exist {
		state.Phase = provisioningv1.BfbReady
		return *state, nil
	}

	if bfbDownloader, ok := butil.DownloadingTaskMap.Load(bfbTaskName); ok {
		// Check whether downloading is finished
		result := bfbDownloader.(*future.Future)
		if result.GetState() != future.Ready {
			return *state, nil
		}
		butil.DownloadingTaskMap.Delete(bfbTaskName)
		butil.DownloadingTaskMap.Delete(bfbTaskName + "cancel")
		if _, err := result.GetResult(); err == nil {
			if md5Value, md5err := cutil.ComputeMD5(bfbFilePath); md5err == nil {
				logger.V(3).Info(fmt.Sprintf("md5sum of %s is %s", st.bfb.Spec.FileName, md5Value))
			} else {
				logger.Error(md5err, "Failed to get md5sum")
			}
			state.Phase = provisioningv1.BfbReady
			return *state, nil
		} else if errors.Is(err, context.Canceled) {
			state.Phase = provisioningv1.BfbDeleting
			return *state, nil
		} else {
			state.Phase = provisioningv1.BfbError
			return *state, err
		}
	} else {
		// Bfb downloading
		bfbTask := butil.BfbTask{
			TaskName: bfbTaskName,
			Url:      st.bfb.Spec.URL,
			FileName: st.bfb.Spec.FileName,
			Cfg:      st.bfb.Spec.BFCFG,
		}
		// Create a new context for this download task
		taskCtx, cancel := context.WithCancel(ctx)
		// Store the cancel function in the map
		butil.DownloadingTaskMap.Store(bfbTaskName+"cancel", cancel)
		// Start the download with the new context
		downloadBfb(taskCtx, bfbTask)
	}

	return *state, nil
}

func downloadBfb(ctx context.Context, bfbTask butil.BfbTask) {
	bfbDownloader := future.New(func() (any, error) {
		logger := log.FromContext(ctx)
		logger.V(3).Info("BfbPackage", "start downloading", bfbTask.FileName)

		// Create a temporary file
		tempFile, err := os.CreateTemp(string(os.PathSeparator)+cutil.BFBBaseDir, "bfb-*.tmp")
		if err != nil {
			return nil, err
		}
		defer os.Remove(tempFile.Name()) //nolint: errcheck

		req, err := http.NewRequestWithContext(ctx, "GET", bfbTask.Url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to get: %s status: %d", bfbTask.Url, resp.StatusCode)
		}
		defer resp.Body.Close() //nolint: errcheck

		buf := make([]byte, 128*1024*1024)
	copyLoop:
		for {
			select {
			case <-ctx.Done():
				logger.V(3).Info("BfbPackage", "context canceled", bfbTask.FileName)
				return nil, nil
			default:
				n, err := resp.Body.Read(buf)
				if err != nil && err != io.EOF {
					if errors.Is(err, context.Canceled) {
						return nil, ctx.Err()
					}
					return nil, fmt.Errorf("failed to read from source file: %w", err)
				}
				if n == 0 {
					break copyLoop
				}
				if _, writeErr := tempFile.Write(buf[:n]); writeErr != nil {
					return nil, writeErr
				}
			}
		}

		// Close the temp file before renaming
		if err := tempFile.Close(); err != nil {
			return nil, err
		}

		// Rename the temp file to the final destination
		bfbfile := cutil.GenerateBFBFilePath(bfbTask.FileName)
		if err := os.Rename(tempFile.Name(), bfbfile); err != nil {
			return nil, err
		}

		if err := os.Chmod(bfbfile, 0644); err != nil {
			return nil, err
		}

		logger.V(3).Info("createBfbPackage", "finish", bfbTask.FileName)
		return true, nil
	})
	butil.DownloadingTaskMap.Store(bfbTask.TaskName, bfbDownloader)
}

func IsBfbExist(ctx context.Context, fileName string) (bool, error) {
	logger := log.FromContext(ctx)
	bfbFilePath := cutil.GenerateBFBFilePath(fileName)
	_, err := os.Stat(bfbFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	if md5Value, md5err := cutil.ComputeMD5(bfbFilePath); md5err == nil {
		logger.V(3).Info(fmt.Sprintf("md5sum of %s is %s", fileName, md5Value))
		return true, nil
	} else {
		return false, fmt.Errorf("Failed to get md5sum %v", md5err)
	}
}
