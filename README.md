# how to run application
`go run main.go`

# how to test load balancer
`hit http://localhost:8080/proxy via browser or postman`
`upon hitting the endpoint repeatedly the loadbalancer will re-route to different servers sequentially`

# how to add server
`hit http://localhost:8080/urls/register via browser or postman POST request`
`sample request body  {"url":"http"} `
