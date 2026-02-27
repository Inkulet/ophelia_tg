import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { SiteSettings } from '../../models/cms.model';
import { CmsService } from '../../services/cms.service';

@Component({
  selector: 'app-home',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './home.component.html',
  styleUrls: ['./home.component.css'],
})
export class HomeComponent implements OnInit {
  settings: SiteSettings | null = null;
  avatarUrl = '';
  loading = true;
  error = '';

  constructor(private readonly cmsService: CmsService) {}

  ngOnInit(): void {
    this.cmsService.getSiteSettings().subscribe({
      next: (settings) => {
        this.settings = settings;
        this.avatarUrl = this.cmsService.resolveMediaUrl(settings.avatarURL);
        this.loading = false;
      },
      error: () => {
        this.error = 'Не удалось загрузить данные главной страницы.';
        this.loading = false;
      },
    });
  }
}
