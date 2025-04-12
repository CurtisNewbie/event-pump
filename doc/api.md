# API Endpoints

## Contents

- [POST /api/v1/create-pipeline](#post-apiv1create-pipeline)
- [POST /api/v1/remove-pipeline](#post-apiv1remove-pipeline)
- [GET /api/v1/list-pipeline](#get-apiv1list-pipeline)
- [GET /auth/resource](#get-authresource)
- [GET /metrics](#get-metrics)

## POST /api/v1/create-pipeline

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

- Miso HTTP Client (experimental, demo may not work):
  ```go
  type ApiPipeline struct {
  	Schema string `json:"schema"`  // schema name
  	Table string `json:"table"`    // table name
  	EventTypes []string `json:"eventTypes"` // event types; INS - Insert, UPD - Update, DEL - Delete
  	Stream string `json:"stream"`  // event bus name
  	Condition Condition `json:"condition"`
  }

  type Condition struct {
  	ColumnChanged []string `json:"columnChanged"`
  }

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

## POST /api/v1/remove-pipeline

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

- Miso HTTP Client (experimental, demo may not work):
  ```go
  type ApiPipeline struct {
  	Schema string `json:"schema"`  // schema name
  	Table string `json:"table"`    // table name
  	EventTypes []string `json:"eventTypes"` // event types; INS - Insert, UPD - Update, DEL - Delete
  	Stream string `json:"stream"`  // event bus name
  	Condition Condition `json:"condition"`
  }

  type Condition struct {
  	ColumnChanged []string `json:"columnChanged"`
  }

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

## GET /api/v1/list-pipeline

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

- Miso HTTP Client (experimental, demo may not work):
  ```go
  type ApiPipeline struct {
  	Schema string `json:"schema"`  // schema name
  	Table string `json:"table"`    // table name
  	EventTypes []string `json:"eventTypes"` // event types; INS - Insert, UPD - Update, DEL - Delete
  	Stream string `json:"stream"`  // event bus name
  	Condition Condition `json:"condition"`
  }

  type Condition struct {
  	ColumnChanged []string `json:"columnChanged"`
  }

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

## GET /auth/resource

- Description: Expose resource and endpoint information to other backend service for authorization.
- Expected Access Scope: PROTECTED
- JSON Response:
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

- Miso HTTP Client (experimental, demo may not work):
  ```go
  type ResourceInfoRes struct {
  	Resources []Resource `json:"resources"`
  	Paths []Endpoint `json:"paths"`
  }

  type Resource struct {
  	Name string `json:"name"`      // resource name
  	Code string `json:"code"`      // resource code, unique identifier
  }

  type Endpoint struct {
  	Type string `json:"type"`      // access scope type: PROTECTED/PUBLIC
  	Url string `json:"url"`        // endpoint url
  	Group string `json:"group"`    // app name
  	Desc string `json:"desc"`      // description of the endpoint
  	ResCode string `json:"resCode"` // resource code
  	Method string `json:"method"`  // http method
  }

  func SendRequest(rail miso.Rail) (ResourceInfoRes, error) {
  	var res miso.GnResp[ResourceInfoRes]
  	err := miso.NewDynTClient(rail, "/auth/resource", "event-pump").
  		Get().
  		Json(&res)
  	if err != nil {
  		rail.Errorf("Request failed, %v", err)
  		var dat ResourceInfoRes
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
    this.http.get<ResourceInfoRes>(`/event-pump/auth/resource`)
      .subscribe({
        next: (resp) => {
        },
        error: (err) => {
          console.log(err)
          this.snackBar.open("Request failed, unknown error", "ok", { duration: 3000 })
        }
      });
  }
  ```

## GET /metrics

- Description: Collect prometheus metrics information
- Header Parameter:
  - "Authorization": Basic authorization if enabled
- cURL:
  ```sh
  curl -X GET 'http://localhost:8088/metrics' \
    -H 'Authorization: '
  ```

- Miso HTTP Client (experimental, demo may not work):
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
