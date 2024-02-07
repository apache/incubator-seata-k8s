package seata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	seatav1alpha1 "github.com/apache/seata-k8s/api/v1alpha1"
	"github.com/apache/seata-k8s/pkg/utils"
	"golang.org/x/sync/errgroup"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"net/http"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func SyncService(curr *apiv1.Service, next *apiv1.Service) {
	curr.Spec.Ports = next.Spec.Ports
}

func SyncStatefulSet(curr *appsv1.StatefulSet, next *appsv1.StatefulSet) {
	curr.Spec.Template = next.Spec.Template
	curr.Spec.Replicas = next.Spec.Replicas
}

type rspData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
	Success bool   `json:"success"`
}

func changeCluster(s *seatav1alpha1.SeataServer, i int32) error {
	client := http.Client{}
	host := fmt.Sprintf("%s-%d.%s.%s.svc:%d", s.Name, i, s.Spec.ServiceName, s.Namespace, s.Spec.Ports.ConsolePort)
	username, ok := s.Spec.Env["console.user.username"]
	if !ok {
		username = "seata"
	}
	password, ok := s.Spec.Env["console.user.password"]
	if !ok {
		password = "seata"
	}

	values := map[string]string{"username": username, "password": password}
	jsonValue, _ := json.Marshal(values)
	loginUrl := fmt.Sprintf("http://%s/api/v1/auth/login", host)
	rsp, err := client.Post(loginUrl, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}

	d := &rspData{}
	var tokenStr string
	if rsp.StatusCode != http.StatusOK {
		return errors.New("login failed")
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(body, &d); err != nil {
		return err
	}
	if !d.Success {
		return errors.New(d.Message)
	}
	tokenStr = d.Data

	targetUrl := fmt.Sprintf("http://%s/metadata/v1/changeCluster?raftClusterStr=%s",
		host, url.QueryEscape(utils.ConcatRaftServerAddress(s)))
	req, _ := http.NewRequest("POST", targetUrl, nil)
	req.Header.Set("Authorization", tokenStr)
	rsp, err = client.Do(req)
	if err != nil {
		return err
	}

	d = &rspData{}
	if rsp.StatusCode != http.StatusOK {
		return errors.New("failed to changeCluster")
	}

	body, err = io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(body, &d); err != nil {
		return err
	}

	if !d.Success {
		return errors.New(d.Message)
	}
	return nil
}

func SyncRaftCluster(ctx context.Context, s *seatav1alpha1.SeataServer) error {
	logger := log.FromContext(ctx)
	group, childContext := errgroup.WithContext(ctx)

	for i := int32(0); i < s.Spec.Replicas; i++ {
		finalI := i
		group.Go(func() error {
			select {
			case <-childContext.Done():
				return nil
			default:
				err := changeCluster(s, finalI)
				if err != nil {
					logger.Error(err, fmt.Sprintf("fail to SyncRaftCluster at %d-th pod", finalI))
				}
				return err
			}
		})
	}
	return group.Wait()
}
