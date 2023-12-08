package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

/*
* Test on Client level, Json unmarshal
* ensuring payload can be parsed correctly
 */
func TestClientGetRates(t *testing.T) {
	rates := Rates{
		Btc: "0.5",
		Eth: "1",
	}
	data := Data{
		Currency: "USD",
		Rates:    rates,
	}
	mockRates := &CoinbaseRates{
		Data: data,
	}

	// should add a negative test, didn't see unmarshal throwing correct error when seeing malfunctional json
	testTable := []struct {
		name             string
		server           *httptest.Server
		expectedResponse *CoinbaseRates
		expectedErr      error
	}{
		{
			name: "success-response",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data": { "currency": "USD", "rates": { "BTC": "0.5", "ETH": "1"}}}`))
			})),
			expectedResponse: mockRates,
			expectedErr:      nil,
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.server.Close()
			client := &CoinbaseClient{}
			resp, err := client.getRates(tc.server.URL)
			if !reflect.DeepEqual(resp, tc.expectedResponse) {
				t.Errorf("expected (%v), got (%v)", tc.expectedResponse, resp)
			}
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected (%v), got (%v)", tc.expectedErr, err)
			}
		})
	}
}

/*
* Test on controller level, returning different HTTP code
 */
func TestController(t *testing.T) {

	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": { "currency": "USD", "rates": { "BTC": "0.5", "ETH": "1"}}}`))
	}))

	//not sure how to supress panic in unit test
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	testTable := []struct {
		name         string
		server       *httptest.Server
		amount       string
		expectedCode int
	}{
		{
			name:         "valid 200",
			server:       successServer,
			amount:       "1000",
			expectedCode: 200,
		},
		{
			name:         "bad amount 400",
			server:       successServer,
			amount:       "100abc",
			expectedCode: 400,
		},
		{
			name:         "bad external call 500",
			server:       failServer,
			amount:       "1000",
			expectedCode: 500,
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &CoinbaseClient{rateUrl: tc.server.URL}
			nopLogger := zap.NewNop().Sugar()
			controller := &SplitController{
				logger: nopLogger,
				client: mockClient,
			}

			engine := gin.Default()
			engine.GET("/73split", controller.splitHandler)

			request, _ := http.NewRequest("GET", "/73split?amount="+tc.amount, nil)

			recorder := httptest.NewRecorder()

			engine.ServeHTTP(recorder, request)

			if recorder.Code != tc.expectedCode {
				t.Errorf("Expected status code %d, got %d", tc.expectedCode, recorder.Code)
			}

		})
	}

}

/*

//Test on backend level, similar idea, skipped for demo
//mocking client, testing backend calculation (business logic), provide different config
func TestGetSpending(t *testing.T){
}
*/
