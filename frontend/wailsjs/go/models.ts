export namespace main {
	
	export class HostKeyInfo {
	    host: string;
	    port: number;
	    keyType: string;
	    fingerprint: string;
	
	    static createFrom(source: any = {}) {
	        return new HostKeyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.keyType = source["keyType"];
	        this.fingerprint = source["fingerprint"];
	    }
	}
	export class TunnelStatus {
	    profileId: string;
	    state: string;
	    message: string;
	    localEndpoint: string;
	    remoteEndpoint: string;
	    activeConnections: number;
	    connectedAt?: string;
	    hostKey?: HostKeyInfo;
	
	    static createFrom(source: any = {}) {
	        return new TunnelStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.state = source["state"];
	        this.message = source["message"];
	        this.localEndpoint = source["localEndpoint"];
	        this.remoteEndpoint = source["remoteEndpoint"];
	        this.activeConnections = source["activeConnections"];
	        this.connectedAt = source["connectedAt"];
	        this.hostKey = this.convertValues(source["hostKey"], HostKeyInfo);
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
	export class ProfileView {
	    id: string;
	    name: string;
	    sshHost: string;
	    sshPort: number;
	    username: string;
	    localBind: string;
	    localPort: number;
	    remoteHost: string;
	    remotePort: number;
	    authMode: string;
	    privateKeyPath?: string;
	    rememberSecret: boolean;
	    autoReconnect: boolean;
	    webService: boolean;
	    webScheme?: string;
	    createdAt: string;
	    updatedAt: string;
	    hasStoredSecret: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfileView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.sshHost = source["sshHost"];
	        this.sshPort = source["sshPort"];
	        this.username = source["username"];
	        this.localBind = source["localBind"];
	        this.localPort = source["localPort"];
	        this.remoteHost = source["remoteHost"];
	        this.remotePort = source["remotePort"];
	        this.authMode = source["authMode"];
	        this.privateKeyPath = source["privateKeyPath"];
	        this.rememberSecret = source["rememberSecret"];
	        this.autoReconnect = source["autoReconnect"];
	        this.webService = source["webService"];
	        this.webScheme = source["webScheme"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.hasStoredSecret = source["hasStoredSecret"];
	    }
	}
	export class BootstrapData {
	    profiles: ProfileView[];
	    statuses: TunnelStatus[];
	    configPath: string;
	    knownHostsPath: string;
	    startupError?: string;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profiles = this.convertValues(source["profiles"], ProfileView);
	        this.statuses = this.convertValues(source["statuses"], TunnelStatus);
	        this.configPath = source["configPath"];
	        this.knownHostsPath = source["knownHostsPath"];
	        this.startupError = source["startupError"];
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

		export class NativeHostRegistrationResult {
	    ok: boolean;
	    code?: string;
	    message?: string;
	    extensionId?: string;
	    manifestPath?: string;
	    binaryPath?: string;

		    static createFrom(source: any = {}) {
	        return new NativeHostRegistrationResult(source);
	    }

		    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.code = source["code"];
	        this.message = source["message"];
	        this.extensionId = source["extensionId"];
	        this.manifestPath = source["manifestPath"];
	        this.binaryPath = source["binaryPath"];
	    }
	}
	export class TunnelProfile {
	    id: string;
	    name: string;
	    sshHost: string;
	    sshPort: number;
	    username: string;
	    localBind: string;
	    localPort: number;
	    remoteHost: string;
	    remotePort: number;
	    authMode: string;
	    privateKeyPath?: string;
	    rememberSecret: boolean;
	    autoReconnect: boolean;
	    webService: boolean;
	    webScheme?: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new TunnelProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.sshHost = source["sshHost"];
	        this.sshPort = source["sshPort"];
	        this.username = source["username"];
	        this.localBind = source["localBind"];
	        this.localPort = source["localPort"];
	        this.remoteHost = source["remoteHost"];
	        this.remotePort = source["remotePort"];
	        this.authMode = source["authMode"];
	        this.privateKeyPath = source["privateKeyPath"];
	        this.rememberSecret = source["rememberSecret"];
	        this.autoReconnect = source["autoReconnect"];
	        this.webService = source["webService"];
	        this.webScheme = source["webScheme"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class OperationResult {
	    ok: boolean;
	    code?: string;
	    message?: string;
	    profile?: TunnelProfile;
	    status?: TunnelStatus;
	    hostKey?: HostKeyInfo;
	    url?: string;
	
	    static createFrom(source: any = {}) {
	        return new OperationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.code = source["code"];
	        this.message = source["message"];
	        this.profile = this.convertValues(source["profile"], TunnelProfile);
	        this.status = this.convertValues(source["status"], TunnelStatus);
	        this.hostKey = this.convertValues(source["hostKey"], HostKeyInfo);
	        this.url = source["url"];
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
	export class ParseCommandResult {
	    ok: boolean;
	    code?: string;
	    message?: string;
	    profile?: TunnelProfile;
	
	    static createFrom(source: any = {}) {
	        return new ParseCommandResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.code = source["code"];
	        this.message = source["message"];
	        this.profile = this.convertValues(source["profile"], TunnelProfile);
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
	
	export class SaveProfileRequest {
	    profile: TunnelProfile;
	    secret: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveProfileRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], TunnelProfile);
	        this.secret = source["secret"];
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
	export class StartTunnelRequest {
	    profileId: string;
	    secret: string;
	
	    static createFrom(source: any = {}) {
	        return new StartTunnelRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.secret = source["secret"];
	    }
	}
	

}
