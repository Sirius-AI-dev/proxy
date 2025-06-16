Uniproxy service is a combine service for handle http request (API Gateway) and run background worker processes

##### Installation from source
```
git clone git@github.com:unibackend/uniproxy.git \
&& cd proxy \
&& make build
```

##### Manual
```
./build/proxy proxy --config=config/default.yml
```
##### Docker compose
```
docker-compose -d
```

### Environments
`DB_DSN` - Master database connection string<br/>
`DOMAIN` - Domain for handle public http queries<br/>
`PORT` - Listen port

### Resources
- Documentation https://unibackend.org/docs