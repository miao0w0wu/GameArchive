export namespace main {
	
	export class AppInfo {
	    appName: string;
	    version: string;
	    stage: string;
	    dataDir: string;
	    dbPath: string;
	    ready: boolean;
	    startupError: string;
	    gameCount: number;
	    categoryCount: number;
	    tagCount: number;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.version = source["version"];
	        this.stage = source["stage"];
	        this.dataDir = source["dataDir"];
	        this.dbPath = source["dbPath"];
	        this.ready = source["ready"];
	        this.startupError = source["startupError"];
	        this.gameCount = source["gameCount"];
	        this.categoryCount = source["categoryCount"];
	        this.tagCount = source["tagCount"];
	    }
	}

}

export namespace models {
	
	export class Category {
	    id: number;
	    name: string;
	    color: string;
	    sortOrder: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Tag {
	    id: number;
	    name: string;
	    color: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Tag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Game {
	    id: number;
	    name: string;
	    exePath: string;
	    processName: string;
	    installDir: string;
	    coverPath: string;
	    categoryId?: number;
	    category?: Category;
	    tags: Tag[];
	    totalSeconds: number;
	    steamAppId: number;
	    steamPlaytimeSeconds: number;
	    // Go type: time
	    steamLastSyncedAt?: any;
	    // Go type: time
	    lastPlayedAt?: any;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.exePath = source["exePath"];
	        this.processName = source["processName"];
	        this.installDir = source["installDir"];
	        this.coverPath = source["coverPath"];
	        this.categoryId = source["categoryId"];
	        this.category = this.convertValues(source["category"], Category);
	        this.tags = this.convertValues(source["tags"], Tag);
	        this.totalSeconds = source["totalSeconds"];
	        this.steamAppId = source["steamAppId"];
	        this.steamPlaytimeSeconds = source["steamPlaytimeSeconds"];
	        this.steamLastSyncedAt = this.convertValues(source["steamLastSyncedAt"], null);
	        this.lastPlayedAt = this.convertValues(source["lastPlayedAt"], null);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Report {
	    id: number;
	    title: string;
	    periodType: string;
	    // Go type: time
	    periodStart?: any;
	    // Go type: time
	    periodEnd?: any;
	    content: string;
	    source: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.periodType = source["periodType"];
	        this.periodStart = this.convertValues(source["periodStart"], null);
	        this.periodEnd = this.convertValues(source["periodEnd"], null);
	        this.content = source["content"];
	        this.source = source["source"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SaveArchive {
	    id: number;
	    gameId: number;
	    name: string;
	    sourcePath: string;
	    backupDir: string;
	    // Go type: time
	    lastBackupAt?: any;
	    note: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new SaveArchive(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.gameId = source["gameId"];
	        this.name = source["name"];
	        this.sourcePath = source["sourcePath"];
	        this.backupDir = source["backupDir"];
	        this.lastBackupAt = this.convertValues(source["lastBackupAt"], null);
	        this.note = source["note"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SaveBackup {
	    id: number;
	    archiveId: number;
	    backupPath: string;
	    // Go type: time
	    backedUpAt: any;
	    note: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new SaveBackup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.archiveId = source["archiveId"];
	        this.backupPath = source["backupPath"];
	        this.backedUpAt = this.convertValues(source["backedUpAt"], null);
	        this.note = source["note"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ScannedGame {
	    name: string;
	    exePath: string;
	    processName: string;
	    installDir: string;
	    sizeBytes: number;
	    suggestedCategory: string;
	    categoryId?: number;
	
	    static createFrom(source: any = {}) {
	        return new ScannedGame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.exePath = source["exePath"];
	        this.processName = source["processName"];
	        this.installDir = source["installDir"];
	        this.sizeBytes = source["sizeBytes"];
	        this.suggestedCategory = source["suggestedCategory"];
	        this.categoryId = source["categoryId"];
	    }
	}

}

export namespace services {
	
	export class AIConfig {
	    baseUrl: string;
	    apiKey: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.model = source["model"];
	    }
	}
	export class AIReportRequest {
	    periodType: string;
	    start: string;
	    end: string;
	    title: string;
	    instruction: string;
	
	    static createFrom(source: any = {}) {
	        return new AIReportRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.periodType = source["periodType"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.title = source["title"];
	        this.instruction = source["instruction"];
	    }
	}
	export class ActiveGame {
	    gameId: number;
	    name: string;
	    // Go type: time
	    startedAt: any;
	    durationSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new ActiveGame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameId = source["gameId"];
	        this.name = source["name"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.durationSeconds = source["durationSeconds"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BreakdownItem {
	    name: string;
	    color: string;
	    seconds: number;
	    percent: number;
	    gameCount: number;
	
	    static createFrom(source: any = {}) {
	        return new BreakdownItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.color = source["color"];
	        this.seconds = source["seconds"];
	        this.percent = source["percent"];
	        this.gameCount = source["gameCount"];
	    }
	}
	export class GameDetail {
	    id: number;
	    name: string;
	    exePath: string;
	    processName: string;
	    installDir: string;
	    coverPath: string;
	    categoryId?: number;
	    category?: models.Category;
	    tags: models.Tag[];
	    totalSeconds: number;
	    steamAppId: number;
	    steamPlaytimeSeconds: number;
	    // Go type: time
	    steamLastSyncedAt?: any;
	    // Go type: time
	    lastPlayedAt?: any;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    tagNames: string[];
	    totalHours: number;
	
	    static createFrom(source: any = {}) {
	        return new GameDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.exePath = source["exePath"];
	        this.processName = source["processName"];
	        this.installDir = source["installDir"];
	        this.coverPath = source["coverPath"];
	        this.categoryId = source["categoryId"];
	        this.category = this.convertValues(source["category"], models.Category);
	        this.tags = this.convertValues(source["tags"], models.Tag);
	        this.totalSeconds = source["totalSeconds"];
	        this.steamAppId = source["steamAppId"];
	        this.steamPlaytimeSeconds = source["steamPlaytimeSeconds"];
	        this.steamLastSyncedAt = this.convertValues(source["steamLastSyncedAt"], null);
	        this.lastPlayedAt = this.convertValues(source["lastPlayedAt"], null);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.tagNames = source["tagNames"];
	        this.totalHours = source["totalHours"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GameRankItem {
	    gameId: number;
	    name: string;
	    categoryName: string;
	    categoryColor: string;
	    seconds: number;
	    percent: number;
	    // Go type: time
	    lastPlayedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new GameRankItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameId = source["gameId"];
	        this.name = source["name"];
	        this.categoryName = source["categoryName"];
	        this.categoryColor = source["categoryColor"];
	        this.seconds = source["seconds"];
	        this.percent = source["percent"];
	        this.lastPlayedAt = this.convertValues(source["lastPlayedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ImportResult {
	    imported: number;
	    skipped: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = source["imported"];
	        this.skipped = source["skipped"];
	    }
	}
	export class MonitorStatus {
	    running: boolean;
	    activeGames: ActiveGame[];
	
	    static createFrom(source: any = {}) {
	        return new MonitorStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.activeGames = this.convertValues(source["activeGames"], ActiveGame);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StatsOverview {
	    totalSeconds: number;
	    todaySeconds: number;
	    weekSeconds: number;
	    monthSeconds: number;
	    activeDays: number;
	    gameCount: number;
	    topGames: GameRankItem[];
	    categories: BreakdownItem[];
	
	    static createFrom(source: any = {}) {
	        return new StatsOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalSeconds = source["totalSeconds"];
	        this.todaySeconds = source["todaySeconds"];
	        this.weekSeconds = source["weekSeconds"];
	        this.monthSeconds = source["monthSeconds"];
	        this.activeDays = source["activeDays"];
	        this.gameCount = source["gameCount"];
	        this.topGames = this.convertValues(source["topGames"], GameRankItem);
	        this.categories = this.convertValues(source["categories"], BreakdownItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TrendPoint {
	    label: string;
	    seconds: number;
	    sessions: number;
	
	    static createFrom(source: any = {}) {
	        return new TrendPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.seconds = source["seconds"];
	        this.sessions = source["sessions"];
	    }
	}
	export class StatsResult {
	    periodType: string;
	    bucketUnit: string;
	    start: string;
	    end: string;
	    totalSeconds: number;
	    sessionCount: number;
	    playedGameCount: number;
	    activeDays: number;
	    dailyAverage: number;
	    trend: TrendPoint[];
	    categories: BreakdownItem[];
	    tags: BreakdownItem[];
	    topGames: GameRankItem[];
	
	    static createFrom(source: any = {}) {
	        return new StatsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.periodType = source["periodType"];
	        this.bucketUnit = source["bucketUnit"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.totalSeconds = source["totalSeconds"];
	        this.sessionCount = source["sessionCount"];
	        this.playedGameCount = source["playedGameCount"];
	        this.activeDays = source["activeDays"];
	        this.dailyAverage = source["dailyAverage"];
	        this.trend = this.convertValues(source["trend"], TrendPoint);
	        this.categories = this.convertValues(source["categories"], BreakdownItem);
	        this.tags = this.convertValues(source["tags"], BreakdownItem);
	        this.topGames = this.convertValues(source["topGames"], GameRankItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SteamConfig {
	    apiKey: string;
	    steamId: string;
	
	    static createFrom(source: any = {}) {
	        return new SteamConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.steamId = source["steamId"];
	    }
	}
	export class SteamSyncResult {
	    fetchedGames: number;
	    matchedByAppId: number;
	    matchedByName: number;
	    importedGames: number;
	    totalPlaytimeSeconds: number;
	    // Go type: time
	    syncedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new SteamSyncResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fetchedGames = source["fetchedGames"];
	        this.matchedByAppId = source["matchedByAppId"];
	        this.matchedByName = source["matchedByName"];
	        this.importedGames = source["importedGames"];
	        this.totalPlaytimeSeconds = source["totalPlaytimeSeconds"];
	        this.syncedAt = this.convertValues(source["syncedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SteamSyncStatus {
	    lastAttemptAt: string;
	    lastError: string;
	    lastResult?: SteamSyncResult;
	
	    static createFrom(source: any = {}) {
	        return new SteamSyncStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lastAttemptAt = source["lastAttemptAt"];
	        this.lastError = source["lastError"];
	        this.lastResult = this.convertValues(source["lastResult"], SteamSyncResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

