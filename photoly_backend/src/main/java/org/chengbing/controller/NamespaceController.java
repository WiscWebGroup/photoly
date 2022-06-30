package org.chengbing.controller;


import org.chengbing.entity.Namespace;
import org.chengbing.service.INamespaceService;
import org.chengbing.util.Result;
import org.chengbing.util.UserIdentity;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
import java.util.List;

***REMOVED***
 * <p>
 *  前端控制器
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
@RestController
@RequestMapping("/namespace")
public class NamespaceController {

    @Resource
    INamespaceService service;

    @Resource
    UserIdentity verify;

    @GetMapping("/getRoot")
    public Result<Namespace> queryRootNamespace(HttpServletRequest request)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryRootNamespace(userId), 200);
***REMOVED***

    @GetMapping("/getChildren")
    public Result<List<Namespace>> queryNamespace(HttpServletRequest request, Integer parentId)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryNamespace(userId, parentId), 200);
***REMOVED***

    @PostMapping("/updateName")
    public Result<Integer> updateNamespaceName(HttpServletRequest request, @RequestBody Namespace namespace)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updateNamespaceName(userId, namespace);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @PostMapping("/updateParent")
    public Result<Integer> updateNamespaceParent(HttpServletRequest request, @RequestBody Namespace namespace)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updateNamespaceParent(userId, namespace);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @PostMapping("/insert")
    public Result<Integer> insertNamespace(HttpServletRequest request, @RequestBody Namespace namespace)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-1, 403);
        int change = service.insertNamespace(userId, namespace);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @PostMapping("/delete")
    public Result<Integer> deleteNamespace(HttpServletRequest request, Integer nsId)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deleteNamespace(userId, nsId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***
***REMOVED***

