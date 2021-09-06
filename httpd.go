package main

import (
	"fmt"
    "net/http"
	"sort"
	"net"
	"bytes"
	"encoding/json"
)

func httpdaemonize(nodes map[nodeKey]*node) {
	var htmlPage = `
	<!doctype html>
    <html lang="en">
    <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="description" content="">
    <meta name="author" content="landiscover">
    <meta name="generator" content="landiscover">
    <title>Landiscover</title>
	<!-- jQuery -->
	<script   src="https://code.jquery.com/jquery-3.6.0.min.js"   integrity="sha256-/xUj+3OJU5yExlq6GSYGSHk7tPXikynS7ogEvDej/m4="   crossorigin="anonymous"></script>
    <!-- Bootstrap core CSS -->
	<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.1.0/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-KyZXEAg3QhqLMpG8r+8fhAXLRk2vvoC2f3B09zVXn8CA5QIVfZOJ3BCsw2P0p/We" crossorigin="anonymous">
    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.1.0/dist/js/bootstrap.bundle.min.js" integrity="sha384-U1DAWAznBHeqEIlVSCgzq+c9gqGAJn5c/t99JyeKa9xxaYpSvHU5awsuZVVFIhvj" crossorigin="anonymous"></script>
    </head>
    <body>
    <table class="table table-dark" id="rowslist">
    <thead>
    <tr>
      <th scope="col">Last seen</th>
      <th scope="col">Hardware Address</th>
      <th scope="col">IP</th>
      <th scope="col">Vendor</th>
	  <th scope="col">DNS</th>
	  <th scope="col">NetBIOS</th>
	  <th scope="col">multicast DNS</th>
    </tr>
    </thead>
    <tbody>
    </tbody>
    </table>
    <script>
    var i = setInterval(function(){
	jQuery.ajax({
		type:"GET",
		url:"/refresh",
		dataType:"json",
		success:function(data) {
			// clear table
			$('#rowslist tbody').empty();
			data.forEach(function(node){
				// add items
				$('#rowslist tbody').append("<tr><td>"+node.last_seen+"</td><td>"+node.mac_addr+"</td><td>"+node.ip+"</td><td>"+node.vendor+"</td><td>"+node.dns+"</td><td>"+node.nbns+"</td><td>"+node.mdns+"</td></tr>");
			});
		}
	});
    },2000)
   </script>   
   </body>
   </html>
	`

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w,"%s",htmlPage)
    })

	http.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request){
		tableSortBy := "mac"
		tableSortAsc := true

		var tableRows []uiTableRow
 		tableRows = func() []uiTableRow {
			var ret []uiTableRow
			for _, n := range nodes {
				row := uiTableRow{
					id: fmt.Sprintf("%s_%s", n.mac.String(), n.ip.String()),	
					cells: []string{
						n.lastSeen.Format("Jan 2 15:04:05"),
						n.mac.String(),
						n.ip.String(),
						macVendor(n.mac),
						func() string {
							if n.dns == "" {
								return "-"
							}
							return n.dns
						}(),
						func() string {
							if n.nbns == "" {
								return "-"
							}
							return n.nbns
						}(),
						func() string {
							if n.mdns == "" {
								return "-"
							}
							return n.mdns
						}(),
					},
				}
				ret = append(ret, row)
			}
			return ret
		}()

		sort.Slice(tableRows, func(i, j int) bool {
			n := 0
			switch tableSortBy {
			case "last seen":
				n = 0
			case "mac":
				n = 1
			case "ip":
				n = 2
			case "vendor":
				n = 3
			case "dns":
				n = 4
			case "nbns":
				n = 5
			case "mdns":
				n = 6
			}
	
			if tableSortBy == "ip" {
				if tableRows[i].cells[n] != tableRows[j].cells[n] {
					ipa := net.ParseIP(tableRows[i].cells[n])
					ipb := net.ParseIP(tableRows[j].cells[n])
	
					if tableSortAsc {
						return bytes.Compare(ipa, ipb) < 0
					}
					return bytes.Compare(ipa, ipb) >= 0
				}
			} else {
				if tableRows[i].cells[n] != tableRows[j].cells[n] {
					if tableSortAsc {
						return tableRows[i].cells[n] < tableRows[j].cells[n]
					}
					return tableRows[i].cells[n] > tableRows[j].cells[n]
				}
			}
	
			return tableRows[i].cells[2] < tableRows[j].cells[2]
		})

        type JsonResp struct {
			Id string 		`json:"id"`
			Last string 	`json:"last_seen"`
			Mac string 		`json:"mac_addr"`
			Ip string 		`json:"ip"`
			Vendor string 	`json:"vendor"`
			Dns string 		`json:"dns"`
			Nbns string 	`json:"nbns"`
			Mdns string 	`json:"mdns"`
		}
		Rows := []JsonResp{}
		for _, node := range tableRows {
			Rows = append(Rows,JsonResp{Id: node.id, Last: node.cells[0], Mac: node.cells[1], Ip: node.cells[2], Vendor: node.cells[3], Dns: node.cells[4], Nbns: node.cells[5], Mdns: node.cells[6]})
		}
		resp, err := json.Marshal(Rows)
		if err != nil {
			fmt.Printf("Unable to encode json response: %s\n",err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, string(resp))
    })

	http.ListenAndServe(":8090", nil)
}