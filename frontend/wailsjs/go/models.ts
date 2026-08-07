export namespace main {
	
	export class BookInfo {
	    uuid: string;
	    title: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new BookInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uuid = source["uuid"];
	        this.title = source["title"];
	        this.size = source["size"];
	    }
	}
	export class HostStatus {
	    running: boolean;
	    port: number;
	    deviceId: string;
	    nickname: string;
	
	    static createFrom(source: any = {}) {
	        return new HostStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.port = source["port"];
	        this.deviceId = source["deviceId"];
	        this.nickname = source["nickname"];
	    }
	}
	export class LoginResult {
	    userId: number;
	    nickname: string;
	    email: string;
	    isVip: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LoginResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userId = source["userId"];
	        this.nickname = source["nickname"];
	        this.email = source["email"];
	        this.isVip = source["isVip"];
	    }
	}
	export class SessionInfo {
	    loggedIn: boolean;
	    userId: number;
	    nickname: string;
	    email: string;
	    booksDir: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loggedIn = source["loggedIn"];
	        this.userId = source["userId"];
	        this.nickname = source["nickname"];
	        this.email = source["email"];
	        this.booksDir = source["booksDir"];
	    }
	}

}

