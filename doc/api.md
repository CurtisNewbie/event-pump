# API Endpoints

- POST /api/v1/create-pipeline
  - Description: Create new pipeline. Duplicate pipeline is ignored, HA is not supported.
  - JSON Request:
    - "schema": (string) schema name
    - "table": (string) table name
    - "eventTypes": ([]string) event types; INS - Insert, UPD - Update, DEL - Delete
    - "stream": (string) event bus name
    - "condition": (Condition) extra filtering conditions
      - "columnChanged": ([]string) 
  - JSON Response:
    - "errorCode": (string) error code
    - "msg": (string) message
    - "error": (bool) whether the request was successful
  - cURL:
    ```sh
    curl -X POST 'http://localhost:8088/api/v1/create-pipeline' \
      -H 'Content-Type: application/json' \
      -d '{"condition":{"columnChanged":[]},"eventTypes":[],"schema":"","stream":"","table":""}'
    ```

  - Miso HTTP Client:
    ```go
    func ApiCreatePipeline(rail miso.Rail, req ApiPipeline) error {
    	var res miso.GnResp[any]
    	err := miso.NewDynTClient(rail, "/api/v1/create-pipeline", "event-pump").
    		PostJson(req).
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		return err
    	}
    	err = res.Err()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return err
    }
    ```

  - JSON Request Object In TypeScript:
    ```ts
    export interface ApiPipeline {
      schema?: string;               // schema name
      table?: string;                // table name
      eventTypes?: string[];         // event types; INS - Insert, UPD - Update, DEL - Delete
      stream?: string;               // event bus name
      condition?: Condition;
    }

    export interface Condition {
      columnChanged?: string[];
    }
    ```

  - JSON Response Object In TypeScript:
    ```ts
    export interface Resp {
      errorCode?: string;            // error code
      msg?: string;                  // message
      error?: boolean;               // whether the request was successful
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    createPipeline() {
      let req: ApiPipeline | null = null;
      this.http.post<any>(`/event-pump/api/v1/create-pipeline`, req)
        .subscribe({
          next: (resp) => {
            if (resp.error) {
              this.snackBar.open(resp.msg, "ok", { duration: 6000 })
              return;
            }
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```

- POST /api/v1/remove-pipeline
  - Description: Remove existing pipeline. HA is not supported.
  - JSON Request:
    - "schema": (string) schema name
    - "table": (string) table name
    - "eventTypes": ([]string) event types; INS - Insert, UPD - Update, DEL - Delete
    - "stream": (string) event bus name
    - "condition": (Condition) extra filtering conditions
      - "columnChanged": ([]string) 
  - JSON Response:
    - "errorCode": (string) error code
    - "msg": (string) message
    - "error": (bool) whether the request was successful
  - cURL:
    ```sh
    curl -X POST 'http://localhost:8088/api/v1/remove-pipeline' \
      -H 'Content-Type: application/json' \
      -d '{"condition":{"columnChanged":[]},"eventTypes":[],"schema":"","stream":"","table":""}'
    ```

  - Miso HTTP Client:
    ```go
    func ApiRemovePipeline(rail miso.Rail, req ApiPipeline) error {
    	var res miso.GnResp[any]
    	err := miso.NewDynTClient(rail, "/api/v1/remove-pipeline", "event-pump").
    		PostJson(req).
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		return err
    	}
    	err = res.Err()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return err
    }
    ```

  - JSON Request Object In TypeScript:
    ```ts
    export interface ApiPipeline {
      schema?: string;               // schema name
      table?: string;                // table name
      eventTypes?: string[];         // event types; INS - Insert, UPD - Update, DEL - Delete
      stream?: string;               // event bus name
      condition?: Condition;
    }

    export interface Condition {
      columnChanged?: string[];
    }
    ```

  - JSON Response Object In TypeScript:
    ```ts
    export interface Resp {
      errorCode?: string;            // error code
      msg?: string;                  // message
      error?: boolean;               // whether the request was successful
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    removePipeline() {
      let req: ApiPipeline | null = null;
      this.http.post<any>(`/event-pump/api/v1/remove-pipeline`, req)
        .subscribe({
          next: (resp) => {
            if (resp.error) {
              this.snackBar.open(resp.msg, "ok", { duration: 6000 })
              return;
            }
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```

- GET /api/v1/list-pipeline
  - Description: List existing pipeline. HA is not supported.
  - JSON Response:
    - "errorCode": (string) error code
    - "msg": (string) message
    - "error": (bool) whether the request was successful
    - "data": ([]pump.ApiPipeline) response data
      - "schema": (string) schema name
      - "table": (string) table name
      - "eventTypes": ([]string) event types; INS - Insert, UPD - Update, DEL - Delete
      - "stream": (string) event bus name
      - "condition": (Condition) extra filtering conditions
        - "columnChanged": ([]string) 
  - cURL:
    ```sh
    curl -X GET 'http://localhost:8088/api/v1/list-pipeline'
    ```

  - Miso HTTP Client:
    ```go
    func ApiListPipelines(rail miso.Rail) ([]ApiPipeline, error) {
    	var res miso.GnResp[[]ApiPipeline]
    	err := miso.NewDynTClient(rail, "/api/v1/list-pipeline", "event-pump").
    		Get().
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		var dat []ApiPipeline
    		return dat, err
    	}
    	dat, err := res.Res()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return dat, err
    }
    ```

  - JSON Response Object In TypeScript:
    ```ts
    export interface Resp {
      errorCode?: string;            // error code
      msg?: string;                  // message
      error?: boolean;               // whether the request was successful
      data?: ApiPipeline[];
    }

    export interface ApiPipeline {
      schema?: string;               // schema name
      table?: string;                // table name
      eventTypes?: string[];         // event types; INS - Insert, UPD - Update, DEL - Delete
      stream?: string;               // event bus name
      condition?: Condition;
    }

    export interface Condition {
      columnChanged?: string[];
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    listPipelines() {
      this.http.get<any>(`/event-pump/api/v1/list-pipeline`)
        .subscribe({
          next: (resp) => {
            if (resp.error) {
              this.snackBar.open(resp.msg, "ok", { duration: 6000 })
              return;
            }
            let dat: ApiPipeline[] = resp.data;
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```

- GET /auth/resource
  - Description: Expose resource and endpoint information to other backend service for authorization.
  - Expected Access Scope: PROTECTED
  - JSON Response:
    - "errorCode": (string) error code
    - "msg": (string) message
    - "error": (bool) whether the request was successful
    - "data": (ResourceInfoRes) response data
      - "resources": ([]auth.Resource) 
        - "name": (string) resource name
        - "code": (string) resource code, unique identifier
      - "paths": ([]auth.Endpoint) 
        - "type": (string) access scope type: PROTECTED/PUBLIC
        - "url": (string) endpoint url
        - "group": (string) app name
        - "desc": (string) description of the endpoint
        - "resCode": (string) resource code
        - "method": (string) http method
  - cURL:
    ```sh
    curl -X GET 'http://localhost:8088/auth/resource'
    ```

  - Miso HTTP Client:
    ```go
    func SendRequest(rail miso.Rail) (GnResp, error) {
    	var res miso.GnResp[GnResp]
    	err := miso.NewDynTClient(rail, "/auth/resource", "event-pump").
    		Get().
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		var dat GnResp
    		return dat, err
    	}
    	dat, err := res.Res()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return dat, err
    }
    ```

  - JSON Response Object In TypeScript:
    ```ts
    export interface GnResp {
      errorCode?: string;            // error code
      msg?: string;                  // message
      error?: boolean;               // whether the request was successful
      data?: ResourceInfoRes;
    }

    export interface ResourceInfoRes {
      resources?: Resource[];
      paths?: Endpoint[];
    }

    export interface Resource {
      name?: string;                 // resource name
      code?: string;                 // resource code, unique identifier
    }

    export interface Endpoint {
      type?: string;                 // access scope type: PROTECTED/PUBLIC
      url?: string;                  // endpoint url
      group?: string;                // app name
      desc?: string;                 // description of the endpoint
      resCode?: string;              // resource code
      method?: string;               // http method
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    sendRequest() {
      this.http.get<any>(`/event-pump/auth/resource`)
        .subscribe({
          next: (resp) => {
            if (resp.error) {
              this.snackBar.open(resp.msg, "ok", { duration: 6000 })
              return;
            }
            let dat: ResourceInfoRes = resp.data;
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```

- GET /metrics
  - Description: Collect prometheus metrics information
  - Header Parameter:
    - "Authorization": Basic authorization if enabled
  - cURL:
    ```sh
    curl -X GET 'http://localhost:8088/metrics' \
      -H 'Authorization: '
    ```

  - Miso HTTP Client:
    ```go
    func SendRequest(rail miso.Rail, authorization string) error {
    	var res miso.GnResp[any]
    	err := miso.NewDynTClient(rail, "/metrics", "event-pump").
    		AddHeader("authorization", authorization).
    		Get().
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		return err
    	}
    	err = res.Err()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return err
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    sendRequest() {
      let authorization: any | null = null;
      this.http.get<any>(`/event-pump/metrics`,
        {
          headers: {
            "Authorization": authorization
          }
        })
        .subscribe({
          next: () => {
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```

- GET /debug/pprof
  - cURL:
    ```sh
    curl -X GET 'http://localhost:8088/debug/pprof'
    ```

  - Miso HTTP Client:
    ```go
    func SendRequest(rail miso.Rail) error {
    	var res miso.GnResp[any]
    	err := miso.NewDynTClient(rail, "/debug/pprof", "event-pump").
    		Get().
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		return err
    	}
    	err = res.Err()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return err
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    sendRequest() {
      this.http.get<any>(`/event-pump/debug/pprof`)
        .subscribe({
          next: () => {
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```

- GET /debug/pprof/:name
  - cURL:
    ```sh
    curl -X GET 'http://localhost:8088/debug/pprof/:name'
    ```

  - Miso HTTP Client:
    ```go
    func SendRequest(rail miso.Rail) error {
    	var res miso.GnResp[any]
    	err := miso.NewDynTClient(rail, "/debug/pprof/:name", "event-pump").
    		Get().
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		return err
    	}
    	err = res.Err()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return err
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    sendRequest() {
      this.http.get<any>(`/event-pump/debug/pprof/:name`)
        .subscribe({
          next: () => {
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```

- GET /debug/pprof/cmdline
  - cURL:
    ```sh
    curl -X GET 'http://localhost:8088/debug/pprof/cmdline'
    ```

  - Miso HTTP Client:
    ```go
    func SendRequest(rail miso.Rail) error {
    	var res miso.GnResp[any]
    	err := miso.NewDynTClient(rail, "/debug/pprof/cmdline", "event-pump").
    		Get().
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		return err
    	}
    	err = res.Err()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return err
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    sendRequest() {
      this.http.get<any>(`/event-pump/debug/pprof/cmdline`)
        .subscribe({
          next: () => {
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```

- GET /debug/pprof/profile
  - cURL:
    ```sh
    curl -X GET 'http://localhost:8088/debug/pprof/profile'
    ```

  - Miso HTTP Client:
    ```go
    func SendRequest(rail miso.Rail) error {
    	var res miso.GnResp[any]
    	err := miso.NewDynTClient(rail, "/debug/pprof/profile", "event-pump").
    		Get().
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		return err
    	}
    	err = res.Err()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return err
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    sendRequest() {
      this.http.get<any>(`/event-pump/debug/pprof/profile`)
        .subscribe({
          next: () => {
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```

- GET /debug/pprof/symbol
  - cURL:
    ```sh
    curl -X GET 'http://localhost:8088/debug/pprof/symbol'
    ```

  - Miso HTTP Client:
    ```go
    func SendRequest(rail miso.Rail) error {
    	var res miso.GnResp[any]
    	err := miso.NewDynTClient(rail, "/debug/pprof/symbol", "event-pump").
    		Get().
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		return err
    	}
    	err = res.Err()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return err
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    sendRequest() {
      this.http.get<any>(`/event-pump/debug/pprof/symbol`)
        .subscribe({
          next: () => {
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```

- GET /debug/pprof/trace
  - cURL:
    ```sh
    curl -X GET 'http://localhost:8088/debug/pprof/trace'
    ```

  - Miso HTTP Client:
    ```go
    func SendRequest(rail miso.Rail) error {
    	var res miso.GnResp[any]
    	err := miso.NewDynTClient(rail, "/debug/pprof/trace", "event-pump").
    		Get().
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		return err
    	}
    	err = res.Err()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return err
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    sendRequest() {
      this.http.get<any>(`/event-pump/debug/pprof/trace`)
        .subscribe({
          next: () => {
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```

- GET /doc/api
  - Description: Serve the generated API documentation webpage
  - Expected Access Scope: PUBLIC
  - cURL:
    ```sh
    curl -X GET 'http://localhost:8088/doc/api'
    ```

  - Miso HTTP Client:
    ```go
    func SendRequest(rail miso.Rail) error {
    	var res miso.GnResp[any]
    	err := miso.NewDynTClient(rail, "/doc/api", "event-pump").
    		Get().
    		Json(&res)
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    		return err
    	}
    	err = res.Err()
    	if err != nil {
    		rail.Errorf("Request failed, %v", err)
    	}
    	return err
    }
    ```

  - Angular HttpClient Demo:
    ```ts
    import { MatSnackBar } from "@angular/material/snack-bar";
    import { HttpClient } from "@angular/common/http";

    constructor(
      private snackBar: MatSnackBar,
      private http: HttpClient
    ) {}

    sendRequest() {
      this.http.get<any>(`/event-pump/doc/api`)
        .subscribe({
          next: () => {
          },
          error: (err) => {
            console.log(err)
            this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
          }
        });
    }
    ```
