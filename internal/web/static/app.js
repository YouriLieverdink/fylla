function tabFromPath(path) {
  const routes = { '/tasks': 'tasks', '/schedule': 'schedule', '/status': 'status', '/timeline': 'timeline' };
  return routes[path] || 'timeline';
}

function dashboard() {
  return {
    tab: tabFromPath(window.location.pathname),
    events: [],
    tasks: [],
    schedule: null,
    status: null,
    error: null,
    lastRefresh: null,
    loading: {
      today: false,
      tasks: false,
      schedule: false,
      status: false,
    },
    refreshInterval: null,

    async init() {
      this.fetchToday();
      this.fetchTasks();
      // Fetch data for the initial tab if not timeline/tasks
      if (this.tab === 'schedule') this.fetchSchedule();
      if (this.tab === 'status') this.fetchStatus();

      // Auto-refresh timeline every 60 seconds
      this.refreshInterval = setInterval(() => {
        if (this.tab === 'timeline') {
          this.fetchToday();
        }
      }, 60000);

      // Lazy-load schedule and status on tab switch
      this.$watch('tab', (val) => {
        if (val === 'schedule' && !this.schedule) this.fetchSchedule();
        if (val === 'status' && !this.status) this.fetchStatus();
      });

      // Handle browser back/forward
      window.addEventListener('popstate', () => {
        this.tab = tabFromPath(window.location.pathname);
      });
    },

    navigate(tab) {
      this.tab = tab;
      const path = tab === 'timeline' ? '/' : '/' + tab;
      history.pushState(null, '', path);
    },

    async fetchToday() {
      this.loading.today = true;
      try {
        const resp = await fetch('/api/today');
        if (!resp.ok) throw new Error(await resp.text());
        this.events = await resp.json();
        this.lastRefresh = new Date();
      } catch (e) {
        this.error = 'Failed to load timeline: ' + e.message;
      } finally {
        this.loading.today = false;
      }
    },

    async fetchTasks() {
      this.loading.tasks = true;
      try {
        const resp = await fetch('/api/tasks');
        if (!resp.ok) throw new Error(await resp.text());
        this.tasks = await resp.json();
      } catch (e) {
        this.error = 'Failed to load tasks: ' + e.message;
      } finally {
        this.loading.tasks = false;
      }
    },

    async fetchSchedule() {
      this.loading.schedule = true;
      try {
        const resp = await fetch('/api/schedule');
        if (!resp.ok) throw new Error(await resp.text());
        this.schedule = await resp.json();
      } catch (e) {
        this.error = 'Failed to load schedule: ' + e.message;
      } finally {
        this.loading.schedule = false;
      }
    },

    async fetchStatus() {
      this.loading.status = true;
      try {
        const resp = await fetch('/api/status');
        if (!resp.ok) throw new Error(await resp.text());
        this.status = await resp.json();
      } catch (e) {
        this.error = 'Failed to load status: ' + e.message;
      } finally {
        this.loading.status = false;
      }
    },

    formatTime(isoStr) {
      const d = new Date(isoStr);
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false });
    },

    isCurrentEvent(event) {
      const now = Date.now();
      return now >= new Date(event.start).getTime() && now < new Date(event.end).getTime();
    },

    timeAgo(date) {
      const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
      if (seconds < 10) return 'just now';
      if (seconds < 60) return seconds + 's ago';
      const minutes = Math.floor(seconds / 60);
      return minutes + 'm ago';
    },

    priorityLabel(p) {
      const labels = { 1: 'Highest', 2: 'High', 3: 'Medium', 4: 'Low', 5: 'Lowest' };
      return labels[p] || '—';
    },

    groupByDay(allocations) {
      const groups = [];
      let current = null;
      for (const alloc of allocations) {
        const d = new Date(alloc.start);
        const day = d.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' });
        if (!current || current.day !== day) {
          current = { day, items: [] };
          groups.push(current);
        }
        current.items.push(alloc);
      }
      return groups;
    },
  };
}
