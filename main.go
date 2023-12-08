package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strconv"
)

// payload can be moved to other module
// Coinbase API payload
type Rates struct {
	Btc string `json:"BTC"`
	Eth string `json:"ETH"`
}

type Data struct {
	Currency string `json:"currency"`
	Rates    Rates  `json:"rates"`
}

type CoinbaseRates struct {
	Data Data `json:"data"`
}

// Response API payload
type Spending struct {
	CoinName string  `json:"coin"`
	Rate     float64 `json:"rate"`
	Hold     float64 `json:"hold"`
	Spent    float64 `json:"spent"`
}

type SpendingResponse struct {
	Amount  float64  `json:"amount"`
	Seventy Spending `json:"70%"`
	Thirty  Spending `json:"30%"`
}

// experimenting implementation without class: controller -> backend -> backing system layers, easier to test
type CoinbaseClient struct {
	rateUrl string
}

// cake pattern on stacking dependency
type SplitController struct {
	logger *zap.SugaredLogger
	client *CoinbaseClient
}

//field has to start with Capital, otherwise, json.marshal cannot recognize, bug?

/*
* use Viper to move magic number to config, 12 factor app
* move url to config
* Gracefully handling server crash: verbase logging, defer closing on resources clean up
* basic authentication on header or ID injection, return 401
* Switch on/off debug mode with header to expose 500 internal error for debugging purposes
* having string over float64, in case rounding issue
* integration of SDK for metrics collection
* abstract PORT to env var
* Chao engineering, special action, based on header
 */
func main() {

	//switch env based on env
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer logger.Sync()
	//json logging
	suggar := logger.Sugar()

	//move this hardcoded string to config
	coinbaseClient := &CoinbaseClient{rateUrl: "https://api.coinbase.com/v2/exchange-rates"}
	splitController := new(SplitController)
	splitController.client = coinbaseClient
	splitController.logger = suggar

	router := setupRouter(splitController)

	suggar.Infoln("Server started ....")
	router.Run("localhost:8080")
}

// basic setup of server, route, simplification for testing purposes
func setupRouter(controller *SplitController) *gin.Engine {

	r := gin.Default()
	// pretty bad route name, just for demo
	r.GET("/73split", controller.splitHandler)

	return r
}

// get spiltHandler returns the split on input amount to buy BTC and ETH
// logging in Json for ELK filtering, e.g. group by invalid amount, and then value
func (sc *SplitController) splitHandler(c *gin.Context) {
	amount := c.Query("amount")

	amountF64, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		sc.logger.Infow("Invalid amount",
			"value", amount,
		)
		c.JSON(400, gin.H{"code": 400, "message": "Not a valid amount: " + amount})
	}

	// hiding internal error stack for security reason
	// however, we can extend to have a switch on/off to expose, for debugging purposes
	resp, sErr := sc.client.getSpending(amountF64)
	if sErr != nil {
		sc.logger.Error(sErr)
		c.JSON(500, gin.H{"code": 500, "message": "Internal Server Error"})
	}

	c.IndentedJSON(http.StatusOK, resp)
}

// refactor from passing pointer to return a pointer, for unit testing
func (client *CoinbaseClient) getSpending(amount float64) (*SpendingResponse, error) {

	//have these magic number and variable to be renamed and moved to config
	seventyPct := amount * 70 / 100
	thirtyPct := amount * 30 / 100

	coinRates, err := client.getRates(client.rateUrl)
	//optional to log this message, as may duplicated in bubbled up error message
	if err != nil {
		fmt.Println(err)
		//missed out a return
		//return nil, err
	}

	//assume rate from coinbase are always good in schema
	btcRate, _ := strconv.ParseFloat(coinRates.Data.Rates.Btc, 64)
	ethRate, _ := strconv.ParseFloat(coinRates.Data.Rates.Eth, 64)

	seventyHold := seventyPct * btcRate
	thirtyHold := thirtyPct * ethRate

	response := &SpendingResponse{}

	response.Amount = amount

	response.Seventy = Spending{CoinName: "BTC", Hold: seventyHold, Rate: btcRate, Spent: seventyPct}
	response.Thirty = Spending{CoinName: "ETH", Hold: thirtyHold, Rate: ethRate, Spent: thirtyPct}

	return response, nil
}

/*
* Blocking implementation
* improvement
*  - concurrency with goroutines, channels, futures
*  - caching, have a scheduled puller on rate, in memory vs memcached
*  - make url configurable
*  code generation from API
*  refactor to pass in URL as parameter, for unit testing purposes
 */
func (client *CoinbaseClient) getRates(url string) (*CoinbaseRates, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, ioErr := io.ReadAll(resp.Body)
	if ioErr != nil {
		return nil, ioErr
	}

	//bad var naming...
	m := &CoinbaseRates{}

	//fail to parse do not throw error?
	jErr := json.Unmarshal(body, &m)
	if jErr != nil {
		return nil, jErr
	}

	return m, nil
}
