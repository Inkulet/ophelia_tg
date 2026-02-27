import { CommonModule, isPlatformBrowser } from '@angular/common';
import { Component, HostListener, Inject, OnInit, PLATFORM_ID } from '@angular/core';
import { RouterModule } from '@angular/router';
import { SidebarComponent } from './components/sidebar/sidebar.component';
import { CmsService } from './services/cms.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [CommonModule, RouterModule, SidebarComponent],
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.css'],
})
export class AppComponent implements OnInit {
  isScrolled = false;
  backgroundImageStyle = '';

  constructor(
    @Inject(PLATFORM_ID) private readonly platformId: object,
    private readonly cmsService: CmsService,
  ) {}

  ngOnInit(): void {
    this.captureUserIdFromQuery();
    this.loadBackground();
  }

  @HostListener('window:scroll')
  onWindowScroll(): void {
    this.isScrolled = window.pageYOffset > 60;
  }

  private loadBackground(): void {
    if (!isPlatformBrowser(this.platformId)) {
      return;
    }
    this.cmsService.getSiteSettings().subscribe({
      next: (settings) => {
        const background = this.cmsService.resolveMediaUrl(settings.backgroundURL);
        if (background !== '') {
          this.backgroundImageStyle = `url('${background}')`;
        }
      },
      error: (err) => {
        console.error('Settings load failed', err);
      },
    });
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
