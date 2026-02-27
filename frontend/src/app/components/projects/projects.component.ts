import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { Project } from '../../models/cms.model';
import { CmsService } from '../../services/cms.service';

@Component({
  selector: 'app-projects',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './projects.component.html',
  styleUrls: ['./projects.component.css'],
})
export class ProjectsComponent implements OnInit {
  projects: Project[] = [];
  selectedProject: Project | null = null;
  loading = true;
  error = '';

  constructor(private readonly cmsService: CmsService) {}

  ngOnInit(): void {
    this.cmsService.getProjects().subscribe({
      next: (projects) => {
        this.projects = projects;
        this.loading = false;
      },
      error: () => {
        this.error = 'Не удалось загрузить проекты.';
        this.loading = false;
      },
    });
  }

  openProject(project: Project): void {
    this.selectedProject = project;
  }

  closeProject(): void {
    this.selectedProject = null;
  }

  mediaUrl(path: string): string {
    return this.cmsService.resolveMediaUrl(path);
  }
}
