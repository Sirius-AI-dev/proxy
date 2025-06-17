## What is Proxy Service

The Proxy Service is a GoLang application designed for managing incoming HTTP requests.

It uses a [Proxy Config](https://github.com/Sirius-AI-dev/proxy/tree/main/config) to define API requests and specifies where and under what conditions to direct them.

The Proxy Service then performs the following steps:
1.  Receives the request.
2.  Optionally verifies authorization cookies.
3.  Adds `request_context` (with IP, user-agent, etc.) and other configurable parameters to the request body.
4.  Applies request balancing rules to distribute requests evenly among multiple servers.
5.  Calls the necessary service, such as a NodeJS application or a PostgreSQL function.
6.  Processes the response. If the response contains [Proxy Tasks](https://github.com/Sirius-AI-dev/proxy/wiki#proxy-tasks) (in the `proxy:[]` section), it executes these tasks, either synchronously (waiting for a response) or in the background.

## Key Features

1.  Flexible interaction between various services.
2.  Load balancing across servers.
3.  Multi-threaded transaction execution.
4.  Background mode support ensures fast client response.
5.  Support for EventStream for server-client communication.
6.  Collector for gathering data from multiple transactions with subsequent single transaction updates.
7.  Task scheduler with multiple operating modes.
8.  Enhanced security through supporting a single entry point to services. For example, to work with a database, it's sufficient to create a user with permissions only to run one function, without access to tables.
9.  Collects metrics for further visualization via Grafana and Prometheus.

Detailed description is under [Wiki section](https://github.com/Sirius-AI-dev/proxy/wiki)
