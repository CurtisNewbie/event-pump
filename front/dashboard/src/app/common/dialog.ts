import { Injectable } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { ConfirmDialogComponent } from '../confirm/confirm-dialog.component';

@Injectable({
  providedIn: 'root',
})
export class ConfirmDialog {
  constructor(private dialog: MatDialog) {}

  show(title: string, msg: string[], onConfirm: any, width: string = '500px') {
    this.dialog
      .open(ConfirmDialogComponent, {
        width: width,
        data: {
          title: title,
          msg: msg,
          isNoBtnDisplayed: true,
        },
      })
      .afterClosed()
      .subscribe((confirm) => {
        if (!confirm) {
          return;
        }
        onConfirm();
      });
  }
}
