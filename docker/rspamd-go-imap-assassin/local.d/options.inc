# Use unbound in docker, increase timeout for small local installations
dns {
    timeout = 15s;
    nameserver = "unbound:5053";
}