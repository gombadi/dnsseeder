package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"text/template"
	"time"
)

// startHTTP runs in a goroutine and provides the web interface
// to the dnsseeder
func startHTTP(port string) {

	http.HandleFunc("/dns", dnsHandler)
	http.HandleFunc("/twistee", twisteeHandler)
	http.HandleFunc("/statusRG", statusRGHandler)
	http.HandleFunc("/statusCG", statusCGHandler)
	http.HandleFunc("/statusWG", statusWGHandler)
	http.HandleFunc("/statusNG", statusNGHandler)
	http.HandleFunc("/", emptyHandler)
	// listen only on localhost
	err := http.ListenAndServe("127.0.0.1:"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}

// reflectHandler processes all requests and returns output in the requested format
func dnsHandler(w http.ResponseWriter, r *http.Request) {

	st := time.Now()

	// FIXME - This is ugly code and needs to be cleaned up a lot

	// get v4 std addresses
	v4std := getv4stdRR()
	v4non := getv4nonRR()
	v6std := getv6stdRR()
	v6non := getv6nonRR()

	var v4stdstr, v4nonstr []string
	var v6stdstr, v6nonstr []string

	if x := len(v4std); x > 0 {
		v4stdstr = make([]string, x)
		for k, v := range v4std {
			v4stdstr[k] = v.String()
		}
	} else {
		v4stdstr = []string{"No records Available"}
	}

	if x := len(v4non); x > 0 {
		v4nonstr = make([]string, x)
		for k, v := range v4non {
			v4nonstr[k] = v.String()
		}
	} else {
		v4nonstr = []string{"No records Available"}
	}

	// ipv6
	if x := len(v6std); x > 0 {
		v6stdstr = make([]string, x)
		for k, v := range v6std {
			v6stdstr[k] = v.String()
		}
	} else {
		v6stdstr = []string{"No records Available"}
	}

	if x := len(v6non); x > 0 {
		v6nonstr = make([]string, x)
		for k, v := range v6non {
			v6nonstr[k] = v.String()
		}
	} else {
		v6nonstr = []string{"No records Available"}
	}

	t1 := `
	<center>
	<table border=1>
	  <tr>
	  <th>Standard Ports</th>
	  <th>Non Standard Ports</th>
	  </tr>
	  <tr>
	  <td>
	  `
	t2 := `  {{range .}}
	       {{.}}<br>
	     {{end}}
		 `
	t3 := `
	  </td>
	  <td>
	  `
	t4 := `
	  </td>
	  </tr>
	</table>
	</center>
	`

	writeHeader(w, r)
	fmt.Fprintf(w, "<b>Currently serving the following DNS records</b>")
	fmt.Fprintf(w, "<p><center><b>IPv4</b></center></p>")
	fmt.Fprintf(w, t1)

	t := template.New("v4 template")
	t, err := t.Parse(t2)
	if err != nil {
		log.Printf("error parsing template v4 %v\n", err)
	}
	err = t.Execute(w, v4stdstr)
	if err != nil {
		log.Printf("error executing template v4 %v\n", err)
	}

	fmt.Fprintf(w, t3)

	err = t.Execute(w, v4nonstr)
	if err != nil {
		log.Printf("error executing template v4 non %v\n", err)
	}

	fmt.Fprintf(w, t4)

	// ipv6 records

	fmt.Fprintf(w, "<p><center><b>IPv6</b></center></p>")
	fmt.Fprintf(w, t1)

	err = t.Execute(w, v6stdstr)
	if err != nil {
		log.Printf("error executing template v6 %v\n", err)
	}

	fmt.Fprintf(w, t3)

	err = t.Execute(w, v6nonstr)
	if err != nil {
		log.Printf("error executing template v6 non %v\n", err)
	}

	fmt.Fprintf(w, t4)
	writeFooter(w, r, st)
}

// emptyHandler processes all requests for non-existant urls
func emptyHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Nothing to see here. Move along please\n")
}

func statusRGHandler(w http.ResponseWriter, r *http.Request) {
	statusHandler(w, r, statusRG)
}
func statusCGHandler(w http.ResponseWriter, r *http.Request) {
	statusHandler(w, r, statusCG)
}
func statusWGHandler(w http.ResponseWriter, r *http.Request) {
	statusHandler(w, r, statusWG)
}
func statusNGHandler(w http.ResponseWriter, r *http.Request) {
	statusHandler(w, r, statusNG)
}

type webstatus struct {
	Key   string
	Value string
}

func statusHandler(w http.ResponseWriter, r *http.Request, status uint32) {

	startT := time.Now()

	// gather all the info before writing anything to the remote browser
	ws := generateWebStatus(status)

	st := `
	<center>
	<table border=1>
	  <tr>
	  <th>Twistee</th>
	  <th>Summary</th>
	  </tr>
	     {{range .}}
	  <tr>
	  <td>
	       <a href="/twistee?tw={{.Key}}">{{.Key}}</a>
	  </td>
	  <td>
	       {{.Value}}
	  </td>
	  </tr>
	     {{end}}
	</table>
	</center>
	`

	writeHeader(w, r)

	if len(ws) == 0 {
		fmt.Fprintf(w, "No Twistees found with this status")
	} else {

		switch status {
		case statusRG:
			fmt.Fprintf(w, "<center><b>Twistee Status: statusRG</b></center>")
		case statusCG:
			fmt.Fprintf(w, "<center><b>Twistee Status: statusCG</b></center>")
		case statusWG:
			fmt.Fprintf(w, "<center><b>Twistee Status: statusWG</b></center>")
		case statusNG:
			fmt.Fprintf(w, "<center><b>Twistee Status: statusNG</b></center>")
		}
		t := template.New("Status template")
		t, err := t.Parse(st)
		if err != nil {
			log.Printf("error parsing status template %v\n", err)
		}
		err = t.Execute(w, ws)
		if err != nil {
			log.Printf("error executing status template %v\n", err)
		}

	}

	writeFooter(w, r, startT)

}

// copy Twistee details into a template friendly struct
type webtemplate struct {
	Key            string
	IP             string
	Port           uint16
	Statusstr      string
	Rating         string
	Dnstype        string
	Lastconnect    string
	Lastconnectago string
	Lasttry        string
	Lasttryago     string
	Crawlstart     string
	Crawlstartago  string
	Crawlactive    bool
	Connectfails   uint32
	Version        int32
	Strversion     string
	Services       string
	Lastblock      int32
	Nonstdip       string
}

// reflectHandler processes all requests and returns output in the requested format
func twisteeHandler(w http.ResponseWriter, r *http.Request) {

	st := time.Now()

	twt := `
    <center>
    <table border=1>
      <tr>
      <th>Twistee {{.Key}}</th><th>Details</th>
      </tr>
      <tr><td>IP Address</td><td>{{.IP}}</td></tr>
      <tr><td>Port</td><td>{{.Port}}</td></tr>
      <tr><td>DNS Type</td><td>{{.Dnstype}}</td></tr>
      <tr><td>Non Standard IP</td><td>{{.Nonstdip}}</td></tr>
      <tr><td>Last Connect</td><td>{{.Lastconnect}}<br>{{.Lastconnectago}} ago</td></tr>
      <tr><td>Last Connect Status</td><td>{{.Statusstr}}</td></tr>
      <tr><td>Last Try</td><td>{{.Lasttry}}<br>{{.Lasttryago}} ago</td></tr>
      <tr><td>Crawl Start</td><td>{{.Crawlstart}}<br>{{.Crawlstartago}} ago</td></tr>
      <tr><td>Crawl Active</td><td>{{.Crawlactive}}</td></tr>
      <tr><td>Connection Fails</td><td>{{.Connectfails}}</td></tr>
      <tr><td>Remote Version</td><td>{{.Version}}</td></tr>
      <tr><td>Remote SubVersion</td><td>{{.Strversion}}</td></tr>
      <tr><td>Remote Services</td><td>{{.Services}}</td></tr>
      <tr><td>Remote Last Block</td><td>{{.Lastblock}}</td></tr>
    </table>
    </center>
    `
	s := config.seeder

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	// skip the tw= from the raw query
	k := html.UnescapeString(r.URL.RawQuery[3:])
	writeHeader(w, r)
	if _, ok := s.theList[k]; ok == false {
		fmt.Fprintf(w, "Sorry there is no Twistee with those details\n")
	} else {

		tw := s.theList[k]
		wt := webtemplate{
			IP:             tw.na.IP.String(),
			Port:           tw.na.Port,
			Dnstype:        tw.dns2str(),
			Nonstdip:       tw.nonstdIP.String(),
			Statusstr:      tw.statusStr,
			Lastconnect:    tw.lastConnect.String(),
			Lastconnectago: time.Since(tw.lastConnect).String(),
			Lasttry:        tw.lastTry.String(),
			Lasttryago:     time.Since(tw.lastTry).String(),
			Crawlstart:     tw.crawlStart.String(),
			Crawlstartago:  time.Since(tw.crawlStart).String(),
			Connectfails:   tw.connectFails,
			Crawlactive:    tw.crawlActive,
			Version:        tw.version,
			Strversion:     tw.strVersion,
			Services:       tw.services.String(),
			Lastblock:      tw.lastBlock,
		}

		// display details for the Twistee
		t := template.New("Twistee template")
		t, err := t.Parse(twt)
		if err != nil {
			log.Printf("error parsing Twistee template %v\n", err)
		}
		err = t.Execute(w, wt)
		if err != nil {
			log.Printf("error executing Twistee template %v\n", err)
		}

	}
	writeFooter(w, r, st)
}

// generateWebStatus is given a twistee status and returns a slice of webstatus structures
// ready to be ranged over by an html/template
func generateWebStatus(status uint32) (ws []webstatus) {

	s := config.seeder

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	var valueStr string

	for k, v := range s.theList {
		if v.status != status {
			continue
		}

		switch status {
		case statusRG:
			valueStr = fmt.Sprintf("<b>Fail Count:</b> %v <b>DNS Type:</b> %s",
				v.connectFails,
				v.dns2str())
		case statusCG:
			valueStr = fmt.Sprintf("<b>Remote Version:</b> %v%s <b>Last Block:</b> %v <b>DNS Type:</b> %s",
				v.version,
				v.strVersion,
				v.lastBlock,
				v.dns2str())

		case statusWG:
			valueStr = fmt.Sprintf("<b>Last Try:</b> %s ago <b>Last Status:</b> %s\n",
				time.Since(v.lastTry).String(),
				v.statusStr)

		case statusNG:
			valueStr = fmt.Sprintf("<b>Fail Count:</b> %v <b>Last Try:</b> %s ago <b>Last Status:</b> %s\n",
				v.connectFails,
				time.Since(v.lastTry).String(),
				v.statusStr)

		default:
			valueStr = ""
		}

		ows := webstatus{
			Key:   k,
			Value: valueStr,
		}
		ws = append(ws, ows)
	}

	return ws
}

// genHeader will output the standard header
func writeHeader(w http.ResponseWriter, r *http.Request) {
	// we are using basic and simple html here. No fancy graphics or css
	h := `
    <!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01 Transitional//EN" "http://www.w3.org/TR/html4/loose.dtd">
    <html><head><title>dnsseeder</title></head><body>
	<center>
	<a href="/statusRG">statusRG</a>   
	<a href="/statusCG">statusCG</a>   
	<a href="/statusWG">statusWG</a>   
	<a href="/statusNG">statusNG</a>   
	<a href="/dns">DNS</a>
    <br>
    Current Stats (count/started) 
    <table border=1><tr>
    <td>RG: {{.RG}}/{{.RGS}}</td><td>CG: {{.CG}}/{{.CGS}}</td><td>WG: {{.WG}}/{{.WGS}}</td><td>NG: {{.NG}}/{{.NGS}}</td><td>Total: {{.Total}}</td>
    </tr></table>
	</center>
	<hr>
	`
	t := template.New("Header template")
	t, err := t.Parse(h)
	if err != nil {
		log.Printf("error parsing template %v\n", err)
	}

	counts.mtx.RLock()
	err = t.Execute(w, counts)
	counts.mtx.RUnlock()
	if err != nil {
		log.Printf("error executing template %v\n", err)
	}

}

// genFooter will output the standard footer
func writeFooter(w http.ResponseWriter, r *http.Request, st time.Time) {

	// Footer needs to be exported for template processing to work
	var Footer struct {
		Uptime  string
		Version string
		Rt      string
	}

	f := `
	<hr>
	<center>
	<b>Version:</b> {{.Version}}
	<b>Uptime:</b> {{.Uptime}}
	<b>Request Time:</b> {{.Rt}}
	</center>
	</body></html>
	`
	Footer.Uptime = time.Since(config.seeder.uptime).String()
	Footer.Version = config.version
	Footer.Rt = time.Since(st).String()

	t := template.New("Footer template")
	t, err := t.Parse(f)
	if err != nil {
		log.Printf("error parsing template %v\n", err)
	}
	err = t.Execute(w, Footer)
	if err != nil {
		log.Printf("error executing template %v\n", err)
	}

	if config.verbose {
		log.Printf("status - processed web request: %s %s\n",
			r.RemoteAddr,
			r.RequestURI)
	}
}

/*


 */
