import { CommonModule } from '@angular/common';
import { Component, Inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialogRef } from '@angular/material/dialog';
import { MAT_DIALOG_DATA } from '@angular/material/dialog';
import { MatSnackBarModule } from '@angular/material/snack-bar';
import { MatDialogModule } from '@angular/material/dialog';

export interface ConfirmDialogData {
  title: string;
  msg: string[];
  isNoBtnDisplayed: boolean;
}

@Component({
  standalone: true,
  selector: 'confirm-dialog-component',
  imports: [CommonModule, MatButtonModule, MatSnackBarModule, MatDialogModule],
  template: `
    <h1 mat-dialog-title>{{ data.title }}</h1>
    <div mat-dialog-content>
      <p
        style="text-wrap: pretty; line-break: anywhere"
        *ngFor="let line of data.msg"
      >
        {{ line }}
      </p>
    </div>
    <div mat-dialog-actions class="d-flex justify-content-end">
      <button mat-button [mat-dialog-close]="true">Yes</button>
      <button
        mat-button
        *ngIf="data.isNoBtnDisplayed"
        [mat-dialog-close]="false"
        cdkFocusInitial
      >
        No
      </button>
    </div>
  `,
})
export class ConfirmDialogComponent {
  constructor(
    public dialogRef: MatDialogRef<ConfirmDialogComponent, ConfirmDialogData>,
    @Inject(MAT_DIALOG_DATA) public data: ConfirmDialogData
  ) {}
}
