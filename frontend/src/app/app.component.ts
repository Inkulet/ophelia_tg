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
    this.captureAuthTokenFromQuery();
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
        console.error('Не удалось загрузить фон в app.component', err);
      },
    });
  }

  private captureAuthTokenFromQuery(): void {
    if (!isPlatformBrowser(this.platformId)) {
      return;
    }

    const url = new URL(window.location.href);
    const token = (url.searchParams.get('token') ?? '').trim();
    if (token !== '') {
      localStorage.setItem('ophelia_cms_jwt', token);
    }

    const changed = url.searchParams.has('token');
    if (changed) {
      url.searchParams.delete('token');
      // Cleanup legacy keys from previous auth scheme.
      localStorage.removeItem('ophelia_user_id');
      localStorage.removeItem('telegram_user_id');
      localStorage.removeItem('tg_user_id');
      localStorage.removeItem('user_id');
    }

    if (changed) {
      const query = url.searchParams.toString();
      const nextURL = `${url.pathname}${query ? `?${query}` : ''}${url.hash}`;
      window.history.replaceState({}, document.title, nextURL);
    }
  }
}
