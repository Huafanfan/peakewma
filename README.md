# peakewma

`peakewma` is a load balancing algorithm based on [Peak EWMA (Exponentially Weighted Moving Average)](https://en.wikipedia.org/wiki/Moving_average#Exponential_moving_average). It aims to quickly detect and shift traffic away from busy or high-latency backends in a highly concurrent environment, thereby improving overall response efficiency.

## Features

- **Dynamic Latency Awareness**: Uses EWMA to track backend response times and quickly react to increased latency, especially peak values.
- **Adaptive Routing**: Automatically routes requests to healthier backends, increasing throughput and stability.
- **Extensible**: Provides hooks or functions to customize the balancing strategy for various use cases.

## Background

Traditional load balancing algorithms like Round Robin, Least Connections, or Random may not handle sudden latency spikes effectively. Peak EWMA incorporates exponential weighting to more accurately measure latency and quickly redistribute requests when a backend experiences higher round-trip times (RTT).