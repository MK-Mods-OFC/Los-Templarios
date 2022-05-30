/** @format */

import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { APIService } from './api/api.service';
import { ToastService } from './components/toast/toast.service';
import { UpdateService } from './services/update.service';
import { NO_LOGIN_ROUTES } from './utils/consts';
import LocalStorageUtil from './utils/localstorage';
import { NextLoginRedirect } from './utils/objects';

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.scss'],
})
export class AppComponent implements OnInit {
  title = 'MK-Bot Web Interface';
  isSearch = false;

  private lockSearch = false;

  constructor(
    public toasts: ToastService,
    private router: Router,
    private api: APIService,
    private update: UpdateService
  ) {}

  ngOnInit() {
    const nlr = LocalStorageUtil.get<NextLoginRedirect>('NEXT_LOGIN_REDIRECT');
    const path = window.location.pathname;
    if (
      nlr &&
      nlr.deadline >= Date.now() &&
      !NO_LOGIN_ROUTES.find((r) => path.startsWith(r))
    ) {
      LocalStorageUtil.remove('NEXT_LOGIN_REDIRECT');
      window.location.replace(nlr.destination);
    }

    this.update.check();

    window.onkeydown = async (e: KeyboardEvent) => {
      if (e.ctrlKey && e.key === 'f') {
        if (
          this.lockSearch ||
          !(await this.api.getSelfUser().toPromise())?.id
        ) {
          this.lockSearch = true;
          return;
        }
        e.preventDefault();
        this.isSearch = true;
      }

      if (e.key === 'Escape' && this.isSearch) {
        e.preventDefault;
        this.isSearch = false;
      }
    };
  }

  onSearchNavigate(route: string[]) {
    this.isSearch = false;
    this.router.navigate(route);
  }

  onSearchBgClick(e: MouseEvent) {
    if ((e.target as HTMLElement).id !== 'search-bar-container') return;
    e.preventDefault();
    this.isSearch = false;
  }
}
