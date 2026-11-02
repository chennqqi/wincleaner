export namespace main {
	
	export class cleanResult {
	    empty: boolean;
	    deleted: number;
	    failed: number;
	    freed_text: string;
	    errors: string[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new cleanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.empty = source["empty"];
	        this.deleted = source["deleted"];
	        this.failed = source["failed"];
	        this.freed_text = source["freed_text"];
	        this.errors = source["errors"];
	        this.message = source["message"];
	    }
	}
	export class groupView {
	    id: string;
	    title: string;
	    reason: string;
	    count: number;
	    selected: number;
	    size_text: string;
	    checked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new groupView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.reason = source["reason"];
	        this.count = source["count"];
	        this.selected = source["selected"];
	        this.size_text = source["size_text"];
	        this.checked = source["checked"];
	    }
	}
	export class itemView {
	    index: number;
	    name: string;
	    path: string;
	    kind: string;
	    size_text: string;
	    checked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new itemView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.kind = source["kind"];
	        this.size_text = source["size_text"];
	        this.checked = source["checked"];
	    }
	}
	export class viewState {
	    busy: boolean;
	    status: string;
	    groups: groupView[];
	    items: itemView[];
	    selected_id: string;
	    chosen: number;
	    chosen_text: string;
	    done: number;
	    total: number;
	    logs: string[];
	
	    static createFrom(source: any = {}) {
	        return new viewState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.busy = source["busy"];
	        this.status = source["status"];
	        this.groups = this.convertValues(source["groups"], groupView);
	        this.items = this.convertValues(source["items"], itemView);
	        this.selected_id = source["selected_id"];
	        this.chosen = source["chosen"];
	        this.chosen_text = source["chosen_text"];
	        this.done = source["done"];
	        this.total = source["total"];
	        this.logs = source["logs"];
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

