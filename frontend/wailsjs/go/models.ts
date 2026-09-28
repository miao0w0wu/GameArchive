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

}

export namespace services {
	
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

}

