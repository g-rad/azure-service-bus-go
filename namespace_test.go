package servicebus

//	MIT License
//
//	Copyright (c) Microsoft Corporation. All rights reserved.
//
//	Permission is hereby granted, free of charge, to any person obtaining a copy
//	of this software and associated documentation files (the "Software"), to deal
//	in the Software without restriction, including without limitation the rights
//	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//	copies of the Software, and to permit persons to whom the Software is
//	furnished to do so, subject to the following conditions:
//
//	The above copyright notice and this permission notice shall be included in all
//	copies or substantial portions of the Software.
//
//	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
//	SOFTWARE

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-service-bus-go/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type (
	serviceBusSuite struct {
		test.BaseSuite
	}
)

func TestCreateNamespaceFromConnectionString(t *testing.T) {
	connStr := os.Getenv("SERVICEBUS_CONNECTION_STRING") // `Endpoint=sb://XXXX.servicebus.windows.net/;SharedAccessKeyName=XXXX;SharedAccessKey=XXXX`
	ns, err := NewNamespace(NamespaceWithConnectionString(connStr))
	assert.Nil(t, err)
	assert.Contains(t, connStr, ns.Name)
}

func TestServiceBusSuite(t *testing.T) {
	suite.Run(t, new(serviceBusSuite))
}

// TearDownSuite destroys created resources during the run of the suite
func (suite *serviceBusSuite) TearDownSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	suite.deleteAllTaggedQueues(ctx)
}

func (suite *serviceBusSuite) deleteAllTaggedQueues(ctx context.Context) {
	ns := suite.getNewSasInstance()
	qm := ns.NewQueueManager()

	feed, err := qm.List(ctx)
	if err != nil {
		suite.T().Fatal(err)
	}

	for _, entry := range feed.Entries {
		if strings.HasSuffix(entry.Title, suite.TagID) {
			err := qm.Delete(ctx, entry.Title)
			if err != nil {
				suite.T().Fatal(err)
			}
		}
	}
}

func (suite *serviceBusSuite) getNewSasInstance() *Namespace {
	ns, err := getNewSasInstance(suite.ConnStr)
	if err != nil {
		suite.T().Fatal(err)
	}
	return ns
}

func getNewSasInstance(connStr string) (*Namespace, error) {
	return NewNamespace(NamespaceWithConnectionString(connStr))
}
