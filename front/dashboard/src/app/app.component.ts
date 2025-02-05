import { AfterViewInit, Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterOutlet } from '@angular/router';
import { HttpClient } from '@angular/common/http';
import { MatButtonModule } from '@angular/material/button';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { HttpClientModule } from '@angular/common/http';
import { ConfirmDialog } from './common/dialog';
import { environment } from '../environments/environment';

export interface ApiPipeline {
  schema?: string; // schema name
  table?: string; // table name
  eventTypes?: string[]; // event types; INS - Insert, UPD - Update, DEL - Delete
  stream?: string; // event bus name
  condition?: Condition;
}

export interface Condition {
  columnChanged?: string[];
}

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    CommonModule,
    RouterOutlet,
    MatButtonModule,
    MatSnackBarModule,
    MatTableModule,
    HttpClientModule,
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css',
})
export class AppComponent implements AfterViewInit {
  title = 'dashboard';
  dat: ApiPipeline[] = [];

  constructor(
    private snackBar: MatSnackBar,
    private http: HttpClient,
    private confirmDialog: ConfirmDialog
  ) {}

  ngAfterViewInit(): void {
    this.listPipelines();
  }

  listPipelines() {
    this.http.get<any>(`${environment.baseApi}/api/v1/list-pipeline`).subscribe({
      next: (resp) => {
        if (resp.error) {
          this.snackBar.open(resp.msg, 'ok', { duration: 6000 });
          return;
        }
        let dat: ApiPipeline[] = resp.data;
        this.dat = dat;
        if (this.dat == null) {
          this.dat = [];
        }
      },
      error: (err) => {
        console.log(err);
        this.snackBar.open('Request failed, unknown error', 'ok', {
          duration: 3000,
        });
      },
    });
  }

  removePipeline(req: ApiPipeline) {
    this.confirmDialog.show(
      `Remove pipeline for '${req.schema}.${req.table}'?`,
      [
        `Are you sure you want to remove this pipeline?`,
        `- Table: '${req.schema}.${req.table}'`,
        `- Stream: '${req.stream}'`,
      ],
      () => {
        this.http.post<any>(`${environment.baseApi}/api/v1/remove-pipeline`, req).subscribe({
          next: (resp) => {
            if (resp.error) {
              this.snackBar.open(resp.msg, 'ok', { duration: 6000 });
              return;
            }
            this.listPipelines();
          },
          error: (err) => {
            console.log(err);
            this.snackBar.open('Request failed, unknown error', 'ok', {
              duration: 3000,
            });
          },
        });
      }
    );
  }

  // TODO: UI
  createPipeline(req: ApiPipeline) {
    this.http.post<any>(`${environment.baseApi}/api/v1/create-pipeline`, req).subscribe({
      next: (resp) => {
        if (resp.error) {
          this.snackBar.open(resp.msg, 'ok', { duration: 6000 });
          return;
        }
      },
      error: (err) => {
        console.log(err);
        this.snackBar.open('Request failed, unknown error', 'ok', {
          duration: 3000,
        });
      },
    });
  }
}
