/*
Copyright 2017 The Kubernetes Authors.

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

package source

import (
	"context"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/external-dns/endpoint"
)

// serviceCnameSource is an implementation of Source for Kubernetes service objects.
// It will find all services that are under our jurisdiction, i.e. annotated
// desired hostname and matching or no controller annotation. For each of the
// matched services' entrypoints it will return a corresponding
// Endpoint object. The endpoints with IP addresses will be resolved
// through PTR records to names and be added with CNAME records.
type serviceCnameSource struct {
	client     kubernetes.Interface
	namespace  string
	serviceSrc Source
}

// NewServiceSource creates a new serviceSource with the given config.
func NewServiceCnameSource(ctx context.Context, kubeClient kubernetes.Interface, namespace string, serviceSrc Source) (Source, error) {
	// Wrap service source to avoid reinventing the wheel
	return &serviceCnameSource{
		client:     kubeClient,
		namespace:  namespace,
		serviceSrc: serviceSrc,
	}, nil
}

// Endpoints returns endpoint objects for each service that should be processed.
func (sc *serviceCnameSource) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
	endpoints, err := sc.serviceSrc.Endpoints(ctx)
	if err != nil {
		return nil, err
	}

	for _, ep := range endpoints {
		target := ep.Targets[0]
		cname, err := net.LookupAddr(target)
		if err != nil {
			log.Warn("Error retrieving PTR record, leavig as-is: " + target)
			log.Debug("PTR Lookup error: " + err.Error())
			continue
		}
		log.Debug("Resolved PTR " + ep.DNSName + " -> " + cname[0])
		//CNAME might have a trailing "." which we need to remove
		//because some registrys trim it before they return it.
		//If we leave it in the plan gets stuck in an update loop.
		ep.Targets[0] = strings.TrimSuffix(cname[0], ".")
		ep.RecordType = endpoint.RecordTypeCNAME
	}

	return endpoints, nil
}

func (sc *serviceCnameSource) AddEventHandler(ctx context.Context, handler func()) {
	log.Debug("Adding event handler for serviceCname")
	sc.serviceSrc.AddEventHandler(ctx, handler)
}
