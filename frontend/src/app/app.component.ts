import { isPlatformBrowser } from '@angular/common';
import { Component, HostListener, Inject, OnInit, PLATFORM_ID } from '@angular/core';
import { RouterModule } from '@angular/router';
import { SidebarComponent } from './components/sidebar/sidebar.component';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterModule, SidebarComponent],
  template: `
    <!-- Видеофон -->
    <div class="video-background">
      <video autoplay loop muted playsinline class="video-bg">
        <source src="../assets/video/fon_video.mov" type="video/quicktime">
      </video>
    </div>

    <!-- Основной контент -->
    <div class="app-container">
      <nav class="navbar">
        <ul>
          <li><a routerLink="/">Главная</a></li>
          <li><a routerLink="/about">О себе</a></li>
          <li><a routerLink="/skills">Экскурсии</a></li>
          <li><a routerLink="/news">Новости</a></li>
          <li><a routerLink="/events">Мероприятия</a></li>
          <li><a routerLink="/projects">Проекты</a></li>
          <li><a routerLink="/contact">Контакты</a></li>
        </ul>
      </nav>

      <div class="main-content-wrapper">
        <app-sidebar [class.scrolled]="isScrolled"></app-sidebar>
        <div class="content">
          <router-outlet></router-outlet>
        </div>
      </div>
    </div>
  `
})
export class AppComponent implements OnInit {
  constructor(@Inject(PLATFORM_ID) private readonly platformId: object) {}

  isScrolled = false;

  ngOnInit(): void {
    this.captureUserIdFromQuery();
  }

  @HostListener('window:scroll', ['$event'])
  onWindowScroll() {
    this.isScrolled = window.pageYOffset > 60;
  }

  private captureUserIdFromQuery(): void {
    if (!isPlatformBrowser(this.platformId)) {
      return;
    }

    const url = new URL(window.location.href);
    const keys = ['user_id', 'tg_user_id', 'telegram_user_id'];

    let userID: number | null = null;
    for (const key of keys) {
      const value = url.searchParams.get(key);
      if (!value) {
        continue;
      }
      const parsed = Number(value);
      if (Number.isInteger(parsed) && parsed > 0) {
        userID = parsed;
        break;
      }
    }

    if (userID !== null) {
      localStorage.setItem('ophelia_user_id', String(userID));
      localStorage.setItem('telegram_user_id', String(userID));
      localStorage.setItem('user_id', String(userID));
    }

    let changed = false;
    for (const key of keys) {
      if (url.searchParams.has(key)) {
        url.searchParams.delete(key);
        changed = true;
      }
    }

    if (changed) {
      const query = url.searchParams.toString();
      const nextURL = `${url.pathname}${query ? `?${query}` : ''}${url.hash}`;
      window.history.replaceState({}, document.title, nextURL);
    }
  }
}
