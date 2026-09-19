/**
 * CineBucket Frontend Client (`app.js`)
 * Handles movie catalogue rendering, poster cards, genre & actor search filters,
 * IMDB/year/alphabetical sorting, floating details modal cards with synopsis,
 * API Gateway presigned URL requests, and real-time download countdown timers.
 */

(function () {
  'use strict';

  // Application State Store
  const state = {
    movies: [],
    filteredMovies: [],
    allGenres: [],
    allActors: [],
    searchQuery: '',
    selectedGenre: '',
    selectedActor: '',
    activeTab: 'all', // 'all', 'search', 'genres', 'top-rated'
    sortBy: 'id-asc', // 'id-asc', 'imdb-desc', 'imdb-asc', 'year-desc', 'year-asc', 'title-asc', 'title-desc'
    activeMovie: null, // Movie object currently open in floating detail modal
    apiGatewayUrl: localStorage.getItem('cinebucket_api_url') || '',
    activeTimers: {}, // Map of movieId -> { intervalId, remainingSeconds, downloadUrl }
  };

  // DOM Elements Selector Map
  const elements = {
    movieGrid: document.getElementById('movie-grid'),
    loadingState: document.getElementById('loading-state'),
    catalogHeading: document.getElementById('catalog-heading'),
    catalogCount: document.getElementById('catalog-count'),
    activeFilterBadge: document.getElementById('active-filter-badge'),
    activeFilterText: document.getElementById('active-filter-text'),
    btnResetFilters: document.getElementById('btn-reset-filters'),
    
    // Search & Filter Controls
    movieSearch: document.getElementById('movie-search'),
    btnClearSearch: document.getElementById('btn-clear-search'),
    genreSelect: document.getElementById('genre-select'),
    actorSelect: document.getElementById('actor-select'),
    sortSelect: document.getElementById('sort-select'),
    genrePillsContainer: document.getElementById('genre-pills-container'),
    
    // Navigation Tabs
    navTabs: document.querySelectorAll('.nav-tab'),
    
    // Floating Movie Detail Modal Card
    detailModal: document.getElementById('detail-modal'),
    detailPoster: document.getElementById('detail-poster'),
    detailImdbBadge: document.getElementById('detail-imdb-badge'),
    detailYear: document.getElementById('detail-year'),
    detailTitle: document.getElementById('detail-modal-title'),
    detailGenres: document.getElementById('detail-genres'),
    detailDescription: document.getElementById('detail-description'),
    detailActors: document.getElementById('detail-actors'),
    detailActionFooter: document.getElementById('detail-action-footer'),
    btnCloseDetail: document.getElementById('btn-close-detail'),
    
    // Config Modal
    configModal: document.getElementById('config-modal'),
    configForm: document.getElementById('config-form'),
    apiUrlInput: document.getElementById('api-url-input'),
    btnOpenConfig: document.getElementById('btn-open-config'),
    btnCloseConfig: document.getElementById('btn-close-config'),
    
    // Expired Link Modal
    expiredModal: document.getElementById('expired-modal'),
    btnCloseExpired: document.getElementById('btn-close-expired'),
    btnAckExpired: document.getElementById('btn-ack-expired'),
  };

  // App Initialization
  document.addEventListener('DOMContentLoaded', () => {
    initApp();
  });

  function initApp() {
    bindEvents();
    if (state.apiGatewayUrl) {
      elements.apiUrlInput.value = state.apiGatewayUrl;
    }
    fetchCatalog();
  }

  // Bind All Event Listeners
  function bindEvents() {
    // Nav Tabs switching
    elements.navTabs.forEach((tabBtn) => {
      tabBtn.addEventListener('click', () => {
        elements.navTabs.forEach((t) => t.classList.remove('active'));
        tabBtn.classList.add('active');
        state.activeTab = tabBtn.dataset.tab;
        
        if (state.activeTab === 'top-rated') {
          state.sortBy = 'imdb-desc';
          elements.sortSelect.value = 'imdb-desc';
        } else if (state.activeTab === 'search') {
          elements.movieSearch.focus();
        } else if (state.activeTab === 'all') {
          // Keep current filters or reset if desired
        }
        
        filterAndRenderCatalog();
      });
    });

    // Search Input
    elements.movieSearch.addEventListener('input', (e) => {
      state.searchQuery = e.target.value;
      elements.btnClearSearch.style.display = state.searchQuery ? 'flex' : 'none';
      filterAndRenderCatalog();
    });

    // Clear Search Button
    elements.btnClearSearch.addEventListener('click', () => {
      elements.movieSearch.value = '';
      state.searchQuery = '';
      elements.btnClearSearch.style.display = 'none';
      filterAndRenderCatalog();
    });

    // Genre Dropdown
    elements.genreSelect.addEventListener('change', (e) => {
      state.selectedGenre = e.target.value;
      updateGenrePillsUI();
      filterAndRenderCatalog();
    });

    // Actor Dropdown
    elements.actorSelect.addEventListener('change', (e) => {
      state.selectedActor = e.target.value;
      filterAndRenderCatalog();
    });

    // Sort Dropdown
    elements.sortSelect.addEventListener('change', (e) => {
      state.sortBy = e.target.value;
      filterAndRenderCatalog();
    });

    // Reset Filters Button
    elements.btnResetFilters.addEventListener('click', () => {
      resetAllFilters();
    });

    // Floating Modal Close
    elements.btnCloseDetail.addEventListener('click', () => closeModal(elements.detailModal));
    elements.detailModal.addEventListener('click', (e) => {
      if (e.target === elements.detailModal) {
        closeModal(elements.detailModal);
      }
    });

    // Config Modal Listeners
    elements.btnOpenConfig.addEventListener('click', () => openModal(elements.configModal));
    elements.btnCloseConfig.addEventListener('click', () => closeModal(elements.configModal));
    elements.configForm.addEventListener('submit', (e) => {
      e.preventDefault();
      let url = elements.apiUrlInput.value.trim().replace(/\/+$/, '');
      state.apiGatewayUrl = url;
      localStorage.setItem('cinebucket_api_url', url);
      closeModal(elements.configModal);
      showNotification('✅ API Gateway endpoint saved!');
    });

    // Expired Link Modal Listeners
    elements.btnCloseExpired.addEventListener('click', () => closeModal(elements.expiredModal));
    elements.btnAckExpired.addEventListener('click', () => closeModal(elements.expiredModal));
  }

  // Fetch Movies Catalogue JSON
  async function fetchCatalog() {
    try {
      const response = await fetch('./movies.json', { cache: 'no-cache' });
      if (!response.ok) {
        throw new Error(`HTTP error ${response.status}`);
      }
      const data = await response.json();
      state.movies = data;
      
      extractGenresAndActors();
      populateDropdowns();
      renderGenrePills();
      filterAndRenderCatalog();
    } catch (err) {
      console.error('Failed to load movies.json:', err);
      renderErrorState('Failed to load movie catalog. Make sure movies.json exists in root/web.');
    }
  }

  // Extract unique genres & actors for dropdowns and pills
  function extractGenresAndActors() {
    const genreSet = new Set();
    const actorSet = new Set();

    state.movies.forEach((m) => {
      if (Array.isArray(m.genre)) {
        m.genre.forEach((g) => genreSet.add(g.trim()));
      }
      if (Array.isArray(m.Actors)) {
        m.Actors.forEach((a) => actorSet.add(a.trim()));
      }
    });

    state.allGenres = Array.from(genreSet).sort();
    state.allActors = Array.from(actorSet).sort();
  }

  // Populate Select Dropdowns
  function populateDropdowns() {
    // Genre Select Options
    elements.genreSelect.innerHTML = `<option value="">🎭 All Genres (${state.allGenres.length})</option>`;
    state.allGenres.forEach((g) => {
      const opt = document.createElement('option');
      opt.value = g;
      opt.textContent = g;
      elements.genreSelect.appendChild(opt);
    });

    // Actor Select Options
    elements.actorSelect.innerHTML = `<option value="">👤 All Actors (${state.allActors.length})</option>`;
    state.allActors.forEach((a) => {
      const opt = document.createElement('option');
      opt.value = a;
      opt.textContent = a;
      elements.actorSelect.appendChild(opt);
    });
  }

  // Render Quick Genre Pills
  function renderGenrePills() {
    elements.genrePillsContainer.innerHTML = '';
    
    // "All" Pill
    const allPill = document.createElement('button');
    allPill.className = `genre-pill ${!state.selectedGenre ? 'active' : ''}`;
    allPill.textContent = 'All Genres';
    allPill.addEventListener('click', () => {
      state.selectedGenre = '';
      elements.genreSelect.value = '';
      updateGenrePillsUI();
      filterAndRenderCatalog();
    });
    elements.genrePillsContainer.appendChild(allPill);

    // Individual Genre Pills
    state.allGenres.forEach((g) => {
      const pill = document.createElement('button');
      pill.className = `genre-pill ${state.selectedGenre === g ? 'active' : ''}`;
      pill.textContent = g;
      pill.addEventListener('click', () => {
        state.selectedGenre = state.selectedGenre === g ? '' : g;
        elements.genreSelect.value = state.selectedGenre;
        updateGenrePillsUI();
        filterAndRenderCatalog();
      });
      elements.genrePillsContainer.appendChild(pill);
    });
  }

  function updateGenrePillsUI() {
    const pills = elements.genrePillsContainer.querySelectorAll('.genre-pill');
    pills.forEach((p) => {
      if (!state.selectedGenre && p.textContent === 'All Genres') {
        p.classList.add('active');
      } else if (p.textContent === state.selectedGenre) {
        p.classList.add('active');
      } else {
        p.classList.remove('active');
      }
    });
  }

  // Reset All Filters
  function resetAllFilters() {
    state.searchQuery = '';
    state.selectedGenre = '';
    state.selectedActor = '';
    state.sortBy = 'id-asc';

    elements.movieSearch.value = '';
    elements.btnClearSearch.style.display = 'none';
    elements.genreSelect.value = '';
    elements.actorSelect.value = '';
    elements.sortSelect.value = 'id-asc';

    updateGenrePillsUI();
    filterAndRenderCatalog();
  }

  // Main Filtering and Sorting Engine
  function filterAndRenderCatalog() {
    const q = state.searchQuery.toLowerCase().trim();

    state.filteredMovies = state.movies.filter((movie) => {
      // 1. Text Search Query (Title, Actors, Genre)
      let matchesQuery = true;
      if (q) {
        const titleMatch = movie.title ? movie.title.toLowerCase().includes(q) : false;
        const genreMatch = Array.isArray(movie.genre) && movie.genre.some((g) => g.toLowerCase().includes(q));
        const actorMatch = Array.isArray(movie.Actors) && movie.Actors.some((a) => a.toLowerCase().includes(q));
        matchesQuery = titleMatch || genreMatch || actorMatch;
      }

      // 2. Genre Select Filter
      let matchesGenre = true;
      if (state.selectedGenre) {
        matchesGenre = Array.isArray(movie.genre) && movie.genre.includes(state.selectedGenre);
      }

      // 3. Actor Select Filter
      let matchesActor = true;
      if (state.selectedActor) {
        matchesActor = Array.isArray(movie.Actors) && movie.Actors.includes(state.selectedActor);
      }

      return matchesQuery && matchesGenre && matchesActor;
    });

    // Apply Sorting
    sortMovies(state.filteredMovies, state.sortBy);

    // Update Filter Active Badge UI
    updateActiveFilterBadge();

    // Render Grid
    renderCatalogGrid();
  }

  // Sort Movies Array
  function sortMovies(list, sortKey) {
    list.sort((a, b) => {
      switch (sortKey) {
        case 'imdb-desc':
          return (parseFloat(b.IMDB) || 0) - (parseFloat(a.IMDB) || 0);
        case 'imdb-asc':
          return (parseFloat(a.IMDB) || 0) - (parseFloat(b.IMDB) || 0);
        case 'year-desc':
          return (parseInt(b.year) || 0) - (parseInt(a.year) || 0);
        case 'year-asc':
          return (parseInt(a.year) || 0) - (parseInt(b.year) || 0);
        case 'title-asc':
          return (a.title || '').localeCompare(b.title || '');
        case 'title-desc':
          return (b.title || '').localeCompare(a.title || '');
        case 'id-asc':
        default:
          return (parseInt(a.id) || 0) - (parseInt(b.id) || 0);
      }
    });
  }

  // Update Active Filter Badge
  function updateActiveFilterBadge() {
    const filters = [];
    if (state.searchQuery) filters.push(`Query: "${state.searchQuery}"`);
    if (state.selectedGenre) filters.push(`Genre: ${state.selectedGenre}`);
    if (state.selectedActor) filters.push(`Actor: ${state.selectedActor}`);

    if (filters.length > 0) {
      elements.activeFilterText.textContent = `Active Filters: ${filters.join(' • ')}`;
      elements.activeFilterBadge.style.display = 'flex';
    } else {
      elements.activeFilterBadge.style.display = 'none';
    }
  }

  // Render Movie Cards to Grid
  function renderCatalogGrid() {
    elements.catalogCount.textContent = `${state.filteredMovies.length} Movies`;
    elements.movieGrid.innerHTML = '';

    if (state.filteredMovies.length === 0) {
      elements.movieGrid.innerHTML = `
        <div class="empty-state">
          <div style="font-size: 3rem; margin-bottom: 0.75rem;">🎬</div>
          <h4 style="font-family: 'Outfit', sans-serif; font-size: 1.3rem; margin-bottom: 0.5rem; color: var(--text-main);">No Movies Found</h4>
          <p style="color: var(--text-muted); font-size: 0.95rem; margin-bottom: 1.25rem;">
            No results match your selected search criteria or actor/genre filters.
          </p>
          <button class="btn-primary" style="max-width: 220px; margin: 0 auto;" onclick="document.getElementById('btn-reset-filters').click()">
            Reset All Filters
          </button>
        </div>
      `;
      return;
    }

    state.filteredMovies.forEach((movie) => {
      const card = createMovieCard(movie);
      elements.movieGrid.appendChild(card);
    });
  }

  // Create Individual Movie Grid Card Node
  function createMovieCard(movie) {
    const card = document.createElement('div');
    card.className = 'movie-card';
    card.id = `movie-card-${movie.id}`;

    const genresHtml = Array.isArray(movie.genre)
      ? movie.genre.map((g) => `<span class="badge-genre">${escapeHtml(g)}</span>`).join('')
      : '';

    const actorsText = Array.isArray(movie.Actors) ? movie.Actors.join(', ') : '';

    const posterSrc = movie.posterUrl || '';

    card.innerHTML = `
      <div class="card-poster-wrapper">
        ${
          posterSrc
            ? `<img src="${escapeHtml(posterSrc)}" alt="${escapeHtml(movie.title)}" class="card-poster-img" loading="lazy" onerror="this.onerror=null; this.parentNode.innerHTML='<div class=\\'card-poster-fallback\\'><div class=\\'card-poster-fallback-icon\\'>🍿</div><div class=\\'card-poster-fallback-title\\'>${escapeHtml(movie.title)}</div></div>';" />`
            : `<div class="card-poster-fallback"><div class="card-poster-fallback-icon">🍿</div><div class="card-poster-fallback-title">${escapeHtml(movie.title)}</div></div>`
        }
        <div class="card-imdb-badge">⭐ ${movie.IMDB || 'N/A'}</div>
      </div>
      <div class="card-content">
        <div>
          <div class="movie-card-header">
            <h4 class="movie-card-title">${escapeHtml(movie.title)}</h4>
            <span class="movie-card-year">${movie.year || ''}</span>
          </div>
          <div class="movie-card-genres" style="margin-top: 0.5rem;">
            ${genresHtml}
          </div>
        </div>
        ${actorsText ? `<div class="movie-card-actors">🎭 ${escapeHtml(actorsText)}</div>` : ''}
        <button class="btn-card-details">
          <span>✨ View Details & Request</span>
        </button>
      </div>
    `;

    // Click on card opens Floating Details Modal
    card.addEventListener('click', () => {
      openMovieDetailModal(movie);
    });

    return card;
  }

  // Open Floating Movie Detail Modal Card (Displays Synopsis + Details)
  function openMovieDetailModal(movie) {
    state.activeMovie = movie;

    // Set Poster Image with fallback
    if (movie.posterUrl) {
      elements.detailPoster.src = movie.posterUrl;
      elements.detailPoster.onerror = () => {
        elements.detailPoster.src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="300" height="450" viewBox="0 0 300 450"><rect width="300" height="450" fill="%230f172a"/><text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" fill="%2394a3b8" font-size="24">🍿 No Poster Available</text></svg>';
      };
    }

    elements.detailImdbBadge.textContent = `⭐ ${movie.IMDB || 'N/A'}`;
    elements.detailYear.textContent = movie.year ? `Released in ${movie.year}` : '';
    elements.detailTitle.textContent = movie.title;
    elements.detailDescription.textContent = movie.description || 'No description available for this movie.';

    // Render Clickable Genre Tags
    elements.detailGenres.innerHTML = '';
    if (Array.isArray(movie.genre)) {
      movie.genre.forEach((g) => {
        const btn = document.createElement('button');
        btn.className = 'tag-pill';
        btn.textContent = `🎭 ${g}`;
        btn.addEventListener('click', () => {
          state.selectedGenre = g;
          elements.genreSelect.value = g;
          updateGenrePillsUI();
          filterAndRenderCatalog();
          closeModal(elements.detailModal);
        });
        elements.detailGenres.appendChild(btn);
      });
    }

    // Render Clickable Actor Tags
    elements.detailActors.innerHTML = '';
    if (Array.isArray(movie.Actors)) {
      movie.Actors.forEach((actor) => {
        const btn = document.createElement('button');
        btn.className = 'tag-pill tag-pill-actor';
        btn.textContent = `👤 ${actor}`;
        btn.addEventListener('click', () => {
          state.selectedActor = actor;
          elements.actorSelect.value = actor;
          filterAndRenderCatalog();
          closeModal(elements.detailModal);
        });
        elements.detailActors.appendChild(btn);
      });
    }

    // Render Download Footer Button or Active Timer
    renderFloatingActionFooter(movie);

    // Open Modal
    openModal(elements.detailModal);
  }

  // Render Action Footer inside Floating Card
  function renderFloatingActionFooter(movie) {
    const timer = state.activeTimers[movie.id];

    if (timer) {
      elements.detailActionFooter.innerHTML = `
        <div class="timer-container" style="margin-bottom: 0.75rem;">
          <div class="timer-label">
            <span class="badge-pulse" style="background: #fbbf24;"></span>
            <span>Ephemeral Link Active</span>
          </div>
          <div class="timer-clock" id="modal-timer-clock-${movie.id}">
            ${formatMMSS(timer.remainingSeconds)}
          </div>
          <div class="timer-subtext">Automatic AWS cloud resource teardown in 10 minutes</div>
        </div>
        <a 
          href="${timer.downloadUrl}" 
          target="_blank" 
          rel="noopener noreferrer" 
          class="btn-primary btn-download"
        >
          <span>⬇️ Download Movie Now</span>
        </a>
      `;

      // Intercept click to verify presigned S3 URL
      const dlLink = elements.detailActionFooter.querySelector('a');
      dlLink.addEventListener('click', (e) => {
        verifyAndDownload(e, movie.id, timer.downloadUrl);
      });
    } else {
      elements.detailActionFooter.innerHTML = `
        <button class="btn-primary" id="btn-request-download-action">
          <span>⚡ Request Ephemeral Cloud Download</span>
        </button>
      `;

      const reqBtn = elements.detailActionFooter.querySelector('#btn-request-download-action');
      reqBtn.addEventListener('click', () => {
        handleRequestDownload(movie, reqBtn);
      });
    }
  }

  // Handle Request Download Click
  async function handleRequestDownload(movie, button) {
    if (!state.apiGatewayUrl) {
      openModal(elements.configModal);
      showNotification('⚠️ Please configure your API Gateway URL first!');
      return;
    }

    button.disabled = true;
    button.innerHTML = `<span>⏳ Requesting S3 Presigned URL...</span>`;

    const endpoint = `${state.apiGatewayUrl}/request-url`;
    const targetFilename = movie.filename || `${movie.title.replace(/[^a-zA-Z0-9.\-_]/g, '_')}.mp4`;

    try {
      const response = await fetch(endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ filename: targetFilename }),
      });

      if (!response.ok) {
        throw new Error(`API Gateway returned status ${response.status}`);
      }

      const data = await response.json();

      if (data.status === 'approved' && data.download_url) {
        startCountdownTimer(movie.id, data.download_url, data.expires_in_seconds || 600);
      } else {
        throw new Error(data.error || 'Request not approved');
      }
    } catch (err) {
      console.error('Request failed:', err);
      button.disabled = false;
      button.innerHTML = `<span>⚡ Request Ephemeral Cloud Download</span>`;
      alert(`Request Failed: ${err.message}\nEnsure local daemon is running and API Gateway URL is correct.`);
    }
  }

  // Start 10-Minute Countdown Timer
  function startCountdownTimer(movieId, downloadUrl, totalSeconds) {
    if (state.activeTimers[movieId]) {
      clearInterval(state.activeTimers[movieId].intervalId);
    }

    state.activeTimers[movieId] = {
      remainingSeconds: totalSeconds,
      downloadUrl: downloadUrl,
      intervalId: null,
    };

    if (state.activeMovie && state.activeMovie.id === movieId) {
      renderFloatingActionFooter(state.activeMovie);
    }

    state.activeTimers[movieId].intervalId = setInterval(() => {
      const timer = state.activeTimers[movieId];
      if (!timer) return;

      timer.remainingSeconds--;

      if (timer.remainingSeconds <= 0) {
        clearInterval(timer.intervalId);
        delete state.activeTimers[movieId];
        handleLinkExpired(movieId);
      } else {
        const modalClock = document.getElementById(`modal-timer-clock-${movieId}`);
        if (modalClock) {
          modalClock.textContent = formatMMSS(timer.remainingSeconds);
        }
      }
    }, 1000);
  }

  // Verify Presigned Link (Intercept 403 / 404 Error on Access)
  async function verifyAndDownload(event, movieId, downloadUrl) {
    event.preventDefault();

    try {
      const headResp = await fetch(downloadUrl, { method: 'HEAD' });
      if (headResp.status === 403 || headResp.status === 404) {
        handleLinkExpired(movieId);
        return;
      }
      window.open(downloadUrl, '_blank');
    } catch (err) {
      window.open(downloadUrl, '_blank');
    }
  }

  // Handle Link Expiration UI state
  function handleLinkExpired(movieId) {
    if (state.activeTimers[movieId]) {
      clearInterval(state.activeTimers[movieId].intervalId);
      delete state.activeTimers[movieId];
    }

    if (state.activeMovie && state.activeMovie.id === movieId) {
      elements.detailActionFooter.innerHTML = `
        <div class="alert-expired">
          <span class="alert-expired-icon">⚠️</span>
          <div>
            <strong>Link Expired & Purged</strong><br>
            Cloud resources deleted. Request owner for a new upload.
          </div>
        </div>
      `;
    }

    openModal(elements.expiredModal);
  }

  // Helper Utilities
  function formatMMSS(seconds) {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  }

  function escapeHtml(str) {
    if (typeof str !== 'string') return str;
    return str
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  function openModal(modal) {
    modal.classList.add('active');
  }

  function closeModal(modal) {
    modal.classList.remove('active');
  }

  function renderErrorState(msg) {
    elements.movieGrid.innerHTML = `
      <div class="empty-state" style="border-color: rgba(239, 68, 68, 0.3);">
        <div style="font-size: 2.5rem; margin-bottom: 0.5rem;">⚠️</div>
        <h4 style="font-family: 'Outfit', sans-serif; font-size: 1.2rem; color: var(--accent-red); margin-bottom: 0.5rem;">Error Loading Catalog</h4>
        <p style="color: var(--text-muted); font-size: 0.9rem;">${escapeHtml(msg)}</p>
      </div>
    `;
  }

  function showNotification(msg) {
    console.log('[CineBucket]', msg);
  }
})();
