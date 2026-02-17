export namespace model {
	
	export class NodeDTO {
	    index: number;
	    name: string;
	    host: string;
	    port: number;
	    scheme: string;
	    region: string;
	    security: string;
	
	    static createFrom(source: any = {}) {
	        return new NodeDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.scheme = source["scheme"];
	        this.region = source["region"];
	        this.security = source["security"];
	    }
	}
	export class RegionRule {
	    Pattern: string;
	    Region: string;
	
	    static createFrom(source: any = {}) {
	        return new RegionRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Pattern = source["Pattern"];
	        this.Region = source["Region"];
	    }
	}
	export class TestSettings {
	    Attempts: number;
	    Threshold: number;
	    Timeout: number;
	    Concurrency: number;
	    RequireAll: boolean;
	    StopOnFail: boolean;
	    Dedup: boolean;
	    Rename: boolean;
	    RenameFmt: string;
	    RegionRules: RegionRule[];
	    ExcludeEnabled: boolean;
	    ExcludeKeywords: string[];
	    LatencyName: boolean;
	    LatencyFmt: string;
	    IPRename: boolean;
	    IPLookupURL: string;
	    IPLookupTimeout: number;
	    IPNameFmt: string;
	    UseCoreTest: boolean;
	    CorePath: string;
	    CoreTestURL: string;
	    CoreStartTimeout: number;
	    UseBatchMode: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TestSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Attempts = source["Attempts"];
	        this.Threshold = source["Threshold"];
	        this.Timeout = source["Timeout"];
	        this.Concurrency = source["Concurrency"];
	        this.RequireAll = source["RequireAll"];
	        this.StopOnFail = source["StopOnFail"];
	        this.Dedup = source["Dedup"];
	        this.Rename = source["Rename"];
	        this.RenameFmt = source["RenameFmt"];
	        this.RegionRules = this.convertValues(source["RegionRules"], RegionRule);
	        this.ExcludeEnabled = source["ExcludeEnabled"];
	        this.ExcludeKeywords = source["ExcludeKeywords"];
	        this.LatencyName = source["LatencyName"];
	        this.LatencyFmt = source["LatencyFmt"];
	        this.IPRename = source["IPRename"];
	        this.IPLookupURL = source["IPLookupURL"];
	        this.IPLookupTimeout = source["IPLookupTimeout"];
	        this.IPNameFmt = source["IPNameFmt"];
	        this.UseCoreTest = source["UseCoreTest"];
	        this.CorePath = source["CorePath"];
	        this.CoreTestURL = source["CoreTestURL"];
	        this.CoreStartTimeout = source["CoreStartTimeout"];
	        this.UseBatchMode = source["UseBatchMode"];
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

