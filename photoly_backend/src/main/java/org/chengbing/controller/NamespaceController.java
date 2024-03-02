package org.chengbing.controller;


import org.chengbing.entity.Namespace;
import org.chengbing.entity.User;
import org.chengbing.service.INamespaceService;
import org.chengbing.service.IUserService;
import org.chengbing.util.Result;
import org.chengbing.util.UserIdentity;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;
import java.io.*;
import java.util.LinkedHashMap;
import java.util.List;

/**
 * <p>
 *  前端控制器
 * </p>
 *
 *  Note: "namespace" is an abstraction of folder to store the photos in. In photoly we don't
 *  actually has folders of folders in the file system for each user. Instead, we store all photos
 *  of one user into its own (one) folder and associates these photos of their respective namespaces
 *  and thus achieve a structure that resembles regular file system.
 *
 *  Note2: in the note, I use "namespace" and "folder" interchangeably , they all mean the "namespace" concept
 *  if not specifically explained.
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@RestController
@RequestMapping("/namespace")
public class NamespaceController {

    @Resource
    INamespaceService service;

    @Resource
    UserIdentity verify;

    @Resource
    IUserService userService;

    /**
     * Function to get the root namespace information of current user (user with the Token).
     *
     * Note: "namespace" is an abstraction of folder to store the photos in. In photoly we don't
     * actually has folders of folders in the file system for each user. Instead, we store all photos
     * of one user into its own (one) folder and associates these photos of their respective namespaces
     * and thus achieve a structure that resembles regular file system.
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @return a result of namespace object
     * example:
     * {
     *     "t": {
     *         "nsId": 521,
     *         "nsName": "/",
     *         "nsParentId": -1,
     *         "userId": 9991
     *     },
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getRoot")
    public Result<Namespace> queryRootNamespace(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryRootNamespace(userId), 200);
    }

    /**
     * Function to get the children namespaces information of current parent folder. Parent folder must be a
     * folder associated with the user of the access token
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param parentId a namespace ID for the parent namespace
     * @return a result of list of namespace objects
     * example:
     * {
     *     "t": [
     *         {
     *             "nsId": 528,
     *             "nsName": "folder1",
     *             "nsParentId": 532,
     *             "userId": 512
     *         },
     *         {
     *             "nsId": 645,
     *             "nsName": "folder2",
     *             "nsParentId": 532,
     *             "userId": 512
     *         }
     *     ],...
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getChildren")
    public Result<List<Namespace>> queryNamespace(HttpServletRequest request, Integer parentId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryNamespace(userId, parentId), 200);
    }

    /**
     * Function to get the name of a namespace given its ID. namespace must be a
     * folder associated with the user of the access token
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param nsId a namespace ID
     * @return a result of namespace name string
     * example:
     * {
     *     "t": "folder1",
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getNameById")
    public Result<String> queryNameById(HttpServletRequest request, Integer nsId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryNameById(nsId), 200);
    }

    /**
     * Function to update the name of a namespace given its ID. namespace must be a
     * folder associated with the user of the access token
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param namespace a namespace object, must have a valid "nsName" and have an "nsId" of its own
     * example:
     * {
     *     "nsName": "folder_change_name1",
     *     "nsId": 709
     * }
     *
     * @return a result of integer to indicate success or not
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/updateName")
    public Result<Integer> updateNamespaceName(HttpServletRequest request, @RequestBody Namespace namespace)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updateNamespaceName(userId, namespace);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to move the namespace to a different parent folder. namespace and parent ns must be
     * folders associated with the user of the access token
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param namespace a namespace object, must have a valid "nsParentId" and have an "nsId" of its own
     * example:
     * {
     *     "nsParentId": 58,
     *     "nsId": 421
     * }
     *
     * @return a result of integer to indicate success or not
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/updateParent")
    public Result<Integer> updateNamespaceParent(HttpServletRequest request, @RequestBody Namespace namespace)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updateNamespaceParent(userId, namespace);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to create a new namespace in a parent folder. parent ns must be
     * folders associated with the user of the access token
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param namespace a namespace object, must have valid "nsName" and "nsParentId" parameters
     * example:
     * {
     *     "nsParentId": 326,
     *     "nsName": "中れ"
     * }
     *
     * @return a result of integer to indicate success or not
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/insert")
    public Result<Integer> insertNamespace(HttpServletRequest request, @RequestBody Namespace namespace)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-1, 403);
        int change = service.insertNamespace(userId, namespace);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to delete a namespace. It has to be a folder associated with the current user.
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param nsId a namespace ID
     *
     * @return a result of integer to indicate success or not
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/delete")
    public Result<Integer> deleteNamespace(HttpServletRequest request, Integer nsId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deleteNamespace(userId, nsId);
        return change >= 0 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to trace the chain or folders until root is reached.
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param nsId the ID of the namespace to go back from
     *
     * @return a result of List of dict that records the chain going up
     * example:
     * {
     *     "t": [
     *         {
     *             "nsId": 123,
     *             "nsName": "/"
     *         },
     *         {
     *             "nsId": 155,
     *             "nsName": "中れ"
     *         }
     *     ],
     *     "msgCode": 200
     * }
     */
    @GetMapping("/trace")
    public Result<List<LinkedHashMap<String, Object>>> traceNamespaceBack(HttpServletRequest request, Integer nsId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        List<LinkedHashMap<String, Object>> res = service.traceNamespaceBack(userId, nsId);
        return res != null ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to download photos in a folder (namespace).
     *
     * @param token the access token of the current user
     * @param response an HttpServletResponse to set the download
     * @param nsId the ID of the namespace to download
     */
    @GetMapping("/downloadNs/{token}")
    public void downloadNs(@PathVariable String token, HttpServletResponse response, Integer nsId)
    {
        Integer userId = verify.verifyUserByToken(token);
        if (userId < 0)
            return;

        User currUser = userService.getById(userId);
        if (currUser != null && currUser.getUuid() != null)
        {
            String zipLoc = service.downloadFolder(userId, nsId, currUser.getUuid());
            InputStream is = null;
            BufferedInputStream bufferedInputStream = null;
            OutputStream outputStream = null;
            byte[] bytes = new byte[1024];
            try {
                is = new FileInputStream(zipLoc);
                bufferedInputStream = new BufferedInputStream(is);

                response.setContentType(MediaType.APPLICATION_OCTET_STREAM.toString());
                response.addHeader("Content-Disposition", "attachment;fileName=download.zip");
                outputStream = response.getOutputStream();
                int length;
                while ((length = bufferedInputStream.read(bytes)) != -1) {
                    outputStream.write(bytes, 0, length);
                }
                outputStream.flush();
            } catch (IOException e) {
                throw new RuntimeException(e);
            } finally {
                if (is != null)
                {
                    try {
                        is.close();
                    } catch (IOException e) {
                        throw new RuntimeException(e);
                    }

                }
                if (bufferedInputStream != null)
                {
                    try {
                        bufferedInputStream.close();
                    } catch (IOException e) {
                        throw new RuntimeException(e);
                    }
                }

                if (outputStream != null)
                {
                    try {
                        outputStream.close();
                    } catch (IOException e) {
                        throw new RuntimeException(e);
                    }
                }
            }
        }
    }
}

