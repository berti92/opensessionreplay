/**
 * Session Recorder Script
 * Include this script on any website to record user sessions
 */
(function() {
    'use strict';
    
    // Configuration
    const CONFIG = {
        apiEndpoint: 'http://localhost:8080/api/sessions',
        batchSize: 50, // Send events in batches
        batchTimeout: 5000, // Send batch after 5 seconds
    };
    
    // Check if rrweb is available
    if (typeof rrweb === 'undefined') {
        return;
    }
    
    class SessionRecorder {
        constructor() {
            this.events = [];
            this.sessionId = this.generateSessionId();
            this.batchTimer = null;
            this.isRecording = false;
            this.init();
        }
        
        generateSessionId() {
            return 'session_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
        }
        
        init() {
            // Start recording
            this.stopFn = rrweb.record({
                emit: (event) => {
                    this.events.push(event);
                    
                    // Send events in batches
                    if (this.events.length >= CONFIG.batchSize) {
                        this.sendBatch();
                    } else if (!this.batchTimer) {
                        this.batchTimer = setTimeout(() => {
                            this.sendBatch();
                        }, CONFIG.batchTimeout);
                    }
                },
                recordCanvas: true,
                collectFonts: true,
                blockClass: 'sr-block',
                ignoreClass: 'sr-ignore',
            });
            
            this.isRecording = true;
            
            // Send initial session metadata
            this.sendSessionMetadata();
            
            // Handle page unload
            window.addEventListener('beforeunload', () => {
                if (this.events.length > 0) {
                    this.sendBatch(true);
                }
            });
            
            // Handle visibility change
            document.addEventListener('visibilitychange', () => {
                if (document.hidden && this.events.length > 0) {
                    this.sendBatch();
                }
            });
        }
        
        sendSessionMetadata() {
            const metadata = {
                sessionId: this.sessionId,
                url: window.location.href,
                title: document.title,
                userAgent: navigator.userAgent,
                timestamp: new Date().toISOString(),
                viewport: {
                    width: window.innerWidth,
                    height: window.innerHeight
                }
            };
            
            this.sendToServer('/api/sessions/metadata', metadata);
        }
        
        sendBatch(isSync = false) {
            if (this.events.length === 0) return;
            
            const batch = {
                sessionId: this.sessionId,
                events: [...this.events],
                timestamp: new Date().toISOString()
            };
            
            this.events = [];
            
            if (this.batchTimer) {
                clearTimeout(this.batchTimer);
                this.batchTimer = null;
            }
            
            this.sendToServer('/api/sessions/events', batch, isSync);
        }
        
        sendToServer(endpoint, data, isSync = false) {
            const url = CONFIG.apiEndpoint.replace('/api/sessions', endpoint);
            
            if (isSync) {
                // Use sendBeacon for synchronous requests (page unload)
                if (navigator.sendBeacon) {
                    const blob = new Blob([JSON.stringify(data)], {
                        type: 'application/json'
                    });
                    navigator.sendBeacon(url, blob);
                    return;
                }
            }
            
            // Regular fetch request
            fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(data)
            }).catch(error => {
            });
        }
        
        stop() {
            if (this.isRecording && this.stopFn) {
                this.stopFn();
                this.isRecording = false;
                
                // Send remaining events
                if (this.events.length > 0) {
                    this.sendBatch();
                }
                
            }
        }
    }
    
    // Auto-start recording when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            window.sessionRecorder = new SessionRecorder();
        });
    } else {
        window.sessionRecorder = new SessionRecorder();
    }
    
})();