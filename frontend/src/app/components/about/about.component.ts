import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { SiteSettings } from '../../models/cms.model';
import { CmsService } from '../../services/cms.service';

@Component({
  selector: 'app-about',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './about.component.html',
  styleUrls: ['./about.component.css'],
})
export class AboutComponent implements OnInit {
  settings: SiteSettings | null = null;
  loading = true;
  error = '';

  constructor(private readonly cmsService: CmsService) {}

  ngOnInit(): void {
    this.cmsService.getSiteSettings().subscribe({
      next: (settings) => {
        this.settings = settings;
        this.loading = false;
      },
      error: () => {
        this.error = 'Не удалось загрузить раздел «О себе».';
        this.loading = false;
      },
    });
  }
}
