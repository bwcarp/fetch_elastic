# fetch_elastic

This is a Nagios-compatible module which will perform a search against Elasticsearch and alert if the number of hits are above the specified values. It takes a json payload compatible with Elastic's REST API. 

```
Usage of ./fetch_elastic: [options] [elastic hosts]
Host format: http://host1:9200
  -I string
        CloudID
  -a string
        API Token
  -c int
        Critical number of hits
  -ca string
        CA Certificate file
  -cf string
        Counter file for persistent errors
  -e    Run as event handler to remove counter file
  -i string
        Elasticsearch Index (default "index")
  -o string
        Custom phrasing for output (default "hits for elastic search.")
  -p string
        Password
  -q string
        JSON file holding the query
  -s int
        Current status according to Icinga. Remove counter file if 'event' and 'OK'
  -u string
        Username
  -w int
        Warning number of hits
```

The command itself is straightforward. Writing the json config for your searches will require an understanding of Elastic beyond just Kibana's query language. I recommend reading their documentation at https://www.elastic.co/guide/en/elasticsearch/reference/current/full-text-queries.html

Here is an example. This query will look for hosts starting with "database" in the name, and check for errors that include postgresql's default port.

```
{
  "size": 1,
  "query": {
    "bool": {
      "must": [
        {
          "match_phrase_prefix": {
            "host.name": {
              "query": "database"
            }
          }
        },
        {
          "query_string": {
            "default_field": "log",
            "query": "ERROR AND 5432"
          }
        },
        {
          "range": {
            "@timestamp": {
              "gte": "now-10m",
              "lte": "now"
            }
          }
        }
      ]
    }
  }
}
```

`match_phrase_prefix` selects the beginning of the host name. `query_string` uses a Lucene type search string. `range` in this case is set to look at the last 10 minutes, and I would accordingly set Nagios or Icinga to check every 10 minutes. For `size` I've put 1, because we don't actually need any of this data; we're going off the total hits value, not counting the number of returned results. By default, total hits will cap at 10,000. You can adjust this if needed, but it will be more taxing on your cluster and will slow the check down.

In this case, I might want to watch for persisting errors but warn right away. Here's an example of how this would run warning the first time and going critical if it sees 5.

```
/usr/lib/nagios/plugins/fetch_elastic -i my_index -j /opt/elastic_queries/example.json -w 1 -c 5 http://myelasticserver:9200
```

### Counter vs current

This check can continue to count across checks and increment from the last check. It stores the count in a file and adds to it on the next run. Removing the counter file will reset. An example of this:

```
/usr/lib/nagios/plugins/fetch_elastic -i my_index -j /opt/elastic_queries/example.json -w 1 -c 5
-cf /var/run/icinga2/cmd/mycounter.bin http://myelasticserver:9200
```

This command can also be used as an event handler when the `-e` flag is specified. In this case, if you **process check result** to **OK** status, the plugin will remove the counter file for you. This will save you the trouble of logging into the montioring server to remove it by hand. Example usage:

```
/usr/lib/nagios/plugins/fetch_elastic -e -s 0 -cf /var/run/icinga2/cmd/mycounter.bin
```