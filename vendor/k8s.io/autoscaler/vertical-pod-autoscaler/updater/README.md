# Vertical Pod Autoscaler - Updater

# Introduction
Updater component for Vertical Pod Autoscaler described in https://github.com/kubernetes/community/pull/338

Updater runs in Kubernetes cluster and decides which pods should be restarted 
based on resources allocation recommendation calculated by Recommender.
If a pod should be updated, Updater will try to evict the pod. 
It respects the pod disruption budget, by using Eviction API to evict pods.
Updater does not perform the actual resources update, but relies on Vertical Pod Autoscaler admission plugin 
to update pod resources when the pod is recreated after eviction.


# Current implementation
Runs in a loop. On one iteration performs:
* Fetching Vertical Pod Autoscaler configuration - using mocked Lister implementation
* Fetching live pods information with current resource allocation.
* For each replicated pod spec fetching resources allocation recommendation - using mock api.
* Recommendations are cached with ttl (specified by a flag)
* For each replicated pods group calculating if pod update is required and how many replicas can be evicted. 
Updater will always allow eviction of at least one pod in replica set. Maximum ratio of evicted replicas is specified by flag.
* Evicting pods if recommended resources significantly vary from the actual resources allocation.
Threshold for evicting pods is specified by flag as percentage of resource that changed (i.e changes smaller than 10% are ignored)
Priority of evictions within a set of replicated pods is proportional to sum of percentages of changes in resources 
(i.e. pod with 15% memory increase 15% cpu decrease recommended will be evicted
before pod with 20% memory increase and no change in cpu)

# Missing parts
* Recommendation API for fetching data from Vertical Pod Autoscaler Recommender.
* Vertical Pod Autoscaler lister for fetching Vertical Pod Autoscaler config.
* Monitoring
