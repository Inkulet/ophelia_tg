import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { SiteSettings } from '../../models/cms.model';
import { CmsService } from '../../services/cms.service';

@Component({
  selector: 'app-contact',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './contact.component.html',
  styleUrls: ['./contact.component.css'],
})
export class ContactComponent implements OnInit {
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
        this.error = 'Не удалось загрузить контакты.';
        this.loading = false;
      },
    });
  }
}
