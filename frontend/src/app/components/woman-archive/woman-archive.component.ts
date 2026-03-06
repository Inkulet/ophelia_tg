import { CommonModule } from '@angular/common';
import { Component, HostListener, OnInit } from '@angular/core';
import { Woman } from '../../models/cms.model';
import { CmsService } from '../../services/cms.service';

@Component({
  selector: 'app-woman-archive',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './woman-archive.component.html',
  styleUrls: ['./woman-archive.component.css'],
})
export class WomanArchiveComponent implements OnInit {
  women: Woman[] = [];
  fields: string[] = [];
  loading = true;
  fieldsLoading = true;
  error = '';
  fieldsError = '';
  page = 1;
  limit = 12;
  total = 0;
  selectedField = '';
  selectedWoman: Woman | null = null;

  constructor(private readonly cmsService: CmsService) {}

  ngOnInit(): void {
    this.loadFields();
    this.loadWomen(1);
  }

  get totalPages(): number {
    if (this.total <= 0) {
      return 1;
    }
    return Math.max(1, Math.ceil(this.total / this.limit));
  }

  get canGoPrev(): boolean {
    return this.page > 1;
  }

  get canGoNext(): boolean {
    return this.page < this.totalPages;
  }

  loadWomen(page: number): void {
    const nextPage = Math.max(1, page);
    this.loading = true;
    this.error = '';

    this.cmsService.getWomen({
      page: nextPage,
      limit: this.limit,
      field: this.selectedField || undefined,
    }).subscribe({
      next: (response) => {
        this.page = nextPage;
        this.women = response.items;
        this.total = response.total;
        this.loading = false;
      },
      error: () => {
        this.error = 'Не удалось загрузить архив женщин.';
        this.loading = false;
      },
    });
  }

  loadFields(): void {
    this.fieldsLoading = true;
    this.fieldsError = '';

    this.cmsService.getFields().subscribe({
      next: (fields) => {
        this.fields = fields;
        this.fieldsLoading = false;
      },
      error: () => {
        this.fieldsError = 'Не удалось загрузить список сфер.';
        this.fieldsLoading = false;
      },
    });
  }

  selectField(field: string): void {
    this.selectedField = field;
    this.loadWomen(1);
  }

  openWoman(woman: Woman): void {
    this.selectedWoman = woman;
  }

  closeModal(): void {
    this.selectedWoman = null;
  }

  onBackdropClick(event: MouseEvent): void {
    if (event.target === event.currentTarget) {
      this.closeModal();
    }
  }

  @HostListener('document:keydown.escape')
  onEscape(): void {
    if (this.selectedWoman) {
      this.closeModal();
    }
  }

  getWomanPhoto(woman: Woman): string {
    if (woman.photoURL.trim() !== '') {
      return this.cmsService.resolveMediaUrl(woman.photoURL);
    }
    return '/assets/photo1.jpeg';
  }

  trackByWomanId(_: number, woman: Woman): number {
    return woman.id;
  }

  trackByField(_: number, field: string): string {
    return field;
  }
}
