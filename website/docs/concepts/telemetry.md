---
title: Telemetry
description: Quidem magni aut exercitationem maxime rerum eos.
---

## Prometheus

Prometheus is a popular open-source monitoring system that has become the de facto standard for monitoring systems in the cloud-native ecosystem. Prometheus follows a pull model, it collects metrics by querying HTTP endpoints that are exposed by the systems and applications being monitored.

Once the application is running and exposing the metrics endpoint, Prometheus can be configured to scrape the endpoint at regular intervals using a scraper. The scraper retrieves the metrics from the endpoint, parses them, and stores them in the Prometheus server for later analysis and visualization. In addition, it is often necessary to implement a discovery system that can identify new applications to scrape during runtime.

To collect metrics from a system or application, the application needs to expose an HTTP endpoint that returns the metrics in a format that Prometheus can understand.

Many blockchain clients expose the Prometheus HTTP endpoint to expose internal metrics.

## Collector

**Vesta** offers an alternative to using external scraping services by integrating its own metrics collector for Prometheus metrics.

In the Plugin descriptions for each blockchain node, it is possible to specify the port and path that defines the HTTP method for Prometheus metrics. If a container has that telemetry configuration and is running, **Vesta** will automatically start collecting metrics from it and aggregating them into a single HTTP metrics endpoint (`localhost:8080`) for easy monitoring and analysis. Additionally, **Vesta** labels each metric with the ID (and alias) of the deployment to which that node belongs.

By default, all deployments expose metrics. However, users can disable this feature during deployment by using the flag `--metrics=false`.

## Telemetry stack

On the **Vesta** repository, we have included a [docker-compose](https://docs.docker.com/compose/) script to migrate the integrated metrics from **Vesta** to Prometheus and Grafana.

```shell-session
$ git clone https://github.com/umbracle/vesta.git
$ cd vesta/telemetry
$ docker-compose up
```
