package org.chengbing.config;
import org.springframework.context.annotation.Configuration;

import javax.servlet.*;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;
import java.io.IOException;
***REMOVED***
 * @Author: Harold澂冰
 * @Date: 2022/6/24 15:09
***REMOVED***
@Configuration
public class CorsConfig implements Filter {
    @Override
    public void doFilter(ServletRequest req, ServletResponse res, FilterChain chain) throws IOException, ServletException {
        HttpServletRequest request = (HttpServletRequest) req;
        HttpServletResponse response = (HttpServletResponse) res;
        response.setHeader("Access-Control-Allow-Origin", "*");
        response.setHeader("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE, PUT, GET");
        response.setHeader("Access-Control-Allow-Credentials", "true");
        response.setHeader("Access-Control-Max-Age", "3600");
        response.setHeader("Access-Control-Allow-Headers", "*");
        if (request.getMethod().equals("OPTIONS")){
            return;
***REMOVED***
        chain.doFilter(req, res);
***REMOVED***
    @Override
    public void init(FilterConfig filterConfig) {***REMOVED***

    @Override
    public void destroy() {***REMOVED***
***REMOVED***
