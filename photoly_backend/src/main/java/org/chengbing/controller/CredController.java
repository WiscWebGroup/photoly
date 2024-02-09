package org.chengbing.controller;


import com.alibaba.fastjson.JSONObject;
import com.alibaba.fastjson.TypeReference;
import org.chengbing.entity.Cred;
import org.chengbing.entity.Namespace;
import org.chengbing.entity.Photo;
import org.chengbing.service.ICredService;
import org.chengbing.service.IPhotoService;
import org.chengbing.util.Result;
import org.chengbing.util.UserIdentity;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.multipart.MultipartFile;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

/**
 * <p>
 *  前端控制器
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@RestController
@RequestMapping("/cred")

// CRD: Create (Update Photo), Query (Get Photo Address, bytes), Delete (Delete Photo)
public class CredController {

    @Resource
    ICredService service;

    @Resource
    IPhotoService photoService;

    @Resource
    UserIdentity verify;

    /**
     * Function to create a new credential
     * @param request an HttpServletRequest which pass in the user's current access token
     * @param cred a credential object which only need to include "authorization"
     *             Note: "CRD" for "authorization" is the largest collection of rights.
     *             Note: rights can be separated (e.g., "RD", "DC", "D", etc.), sequence doesn't matter.
     *             Note: C is for create (upload photo), Q is for query photo, D is for delete photo.
     * example:
     * {
     *     "authorization": "DR"
     * }
     * @return a result of integer. res == 1 and code == 200 if success.
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/create")
    public Result<Integer> createNewAPI(HttpServletRequest request, @RequestBody Cred cred)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int res = service.createNewAPI(userId, cred);
        return res == 1 ? new Result<>(1, 200) : new Result<>(0, 400);
    }

    /**
     * Function to get list of credentials created by the current logged-in user
     * @param request an HttpServletRequest which pass in the user's current access token
     * @return result of list of credentials
     * example:
     * {
     *     "t": [
     *         {
     *             "credId": 353,
     *             "userId": 319,
     *             "token": "71ab07c8-491e-4ff6-821d-8dcb128bc2a3",
     *             "authorization": "RD"
     *         }
     *     ],
     *     "msgCode": 200
     * }
     */
    @GetMapping("/query")
    public Result<List<Cred>> queryAPI(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        List<Cred> returnList = service.queryAPI(userId);
        return returnList == null ? new Result<>(null, 400) : new Result<>(returnList, 200);
    }

    /**
     * Function to update the authorization set of one credential (e.g., CRD -> RD, removing C right)
     * @param request an HttpServletRequest which pass in the user's current access token
     * @param cred a credential object which need to include "authorization" and "credId"
     * example:
     * {
     *     "authorization": "C",
     *     "credId": 332
     * }
     * @return a result of integer. res == 1 and code == 200 if success.
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/update")
    public Result<Integer> updateAPI(HttpServletRequest request, @RequestBody Cred cred)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int res = service.updateAPI(userId, cred);
        return res == 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to delete a credential
     * @param request an HttpServletRequest which pass in the user's current access token
     * @param credId the credential ID of the credential to be deleted
     * @return a result of integer. res == 1 and code == 200 if success.
     * example: see above
     */
    @PostMapping("/delete")
    public Result<Integer> deleteAPI(HttpServletRequest request, Integer credId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int res = service.deleteAPI(userId, credId);
        return res == 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to upload photo using token
     * @param token the credential UUID
     * @param file a photo file
     * @param photo string representation of JSON object of Photo, only "visibility" and "nsId" are needed.
     * example (form-data):
     * { (Key): (Value)
     *   photo: '{"visibility": 0, "nsId": 56}',
     *   file: A FILE HERE
     * }
     * @return null if failed and string of the new photo's UUID if successful
     * example:
     * 9e2ada95-0602-4b4a-a2c6-532294fe4120
     */
    // CRD: Create (Update Photo), Query (Get Photo Address, bytes), Delete (Delete Photo)
    @PostMapping("/uploadPhoto/{token}")
    public String uploadPhoto(@PathVariable String token, MultipartFile file, @RequestParam String photo)
    {
        Photo photo1 = JSONObject.parseObject(photo, Photo.class);
        Integer userId = service.getUserId(token);
        if (userId == null)
            return "Unidentified Token";
        if (service.getAuth(token).contains("C"))
            return photoService.insertPhotoRU(userId, file, photo1);
        else
            return "Wrong Auth for current Token";
    }

    /**
     * Function to upload multiple photos using token
     * @param token the credential UUID
     * @param files multiple files of photo
     * @param photosStr string representation of list of JSON object of Photos, only "visibility" and "nsId" are needed.
     * example (form-data):
     * { (Key): (Value)
     *   photosStr: '[{"visibility": 0, "nsId": 56}, {"visibility": 1, "nsId": 56}]',
     *   file: TWO FILES HERE
     * }
     * @return null if failed and string of a list of new photos' UUID if successful
     * example:
     * [9e2ada95-0602-4b4a-a2c6-532294fe4120, c4e8d612-f3a4-485b-9e4e-c4bf2f072ead]
     */
    @PostMapping("/uploadPhotos/{token}")
    public List<String> uploadPhotos(@PathVariable String token, MultipartFile[] files, @RequestParam String photosStr)
    {
        List<Photo> photos = JSONObject.parseObject(photosStr, new TypeReference<ArrayList<Photo>>(){});
        Integer userId = service.getUserId(token);
        if (userId == null)
            return null;
        if (service.getAuth(token).contains("C"))
            return photoService.insertPhotosRU(userId, files, photos);
        else
            return null;
    }

    /**
     * Function to render a picture using credential and the photo's UUID
     * @param token the credential's UUID
     * @param uuid the desired photo's UUID
     * example (http://server_addr/cred/render/9e2ada95-0602-4b4a-a2c6-532294fe4120. params. key:value):
     * "uuid":"c4e8d612-f3a4-485b-9e4e-c4bf2f072ead"
     * @return bytes of the photo if successful, otherwise null
     * example:
     * null
     */
    @GetMapping(value = "/render/{token}", produces = MediaType.IMAGE_JPEG_VALUE)
    public byte[] renderImage(@PathVariable String token, String uuid)
    {
        Integer userId = service.getUserId(token);
        if (userId == null)
            return null;
        if (service.getAuth(token).contains("R"))
            return service.render(userId, uuid);
        else
            return null;
    }

    /**
     * Function to delete a picture using credential and the photo's UUID
     * @param token the credential's UUID
     * @param uuid the desired photo's UUID
     * example (http://server_addr/cred/deletePhoto/9e2ada95-0602-4b4a-a2c6-532294fe4120. params. key:value):
     * "uuid":"c4e8d612-f3a4-485b-9e4e-c4bf2f072ead"
     * @return <0 if unsuccessful, 1 if successful
     * example:
     * 1
     */
    @PostMapping("/deletePhoto/{token}")
    public Integer deletePhoto(@PathVariable String token, String uuid)
    {
        Integer userId = service.getUserId(token);
        if (userId == null)
            return -1;
        if (service.getAuth(token).contains("D"))
            return service.deletePhoto(userId, uuid);
        else
            return -2;
    }

    /**
     * Function to get user's root namespace information
     * @param token the credential's UUID
     * example
     * http://server_addr/cred/queryRootNamespace/9e2ada95-0602-4b4a-a2c6-532294fe4120
     * @return a Namespace serialized JSON if successful, null if unsuccessful
     * example:
     * {
     *     "nsId": 2226,
     *     "nsName": "/",
     *     "nsParentId": -1,
     *     "userId": 291
     * }
     */
    @GetMapping("/queryRootNamespace/{token}")
    public Namespace queryRootNamespace(@PathVariable String token)
    {
        Integer userId = service.getUserId(token);
        if (userId == null)
            return null;
        if (service.getAuth(token).contains("R"))
            return service.queryRootNamespace(userId);
        else
            return null;
    }

    /**
     * Function to get user's children namespace information
     * @param token the credential's UUID
     * @param parentId the parent namespace's id
     * example (http://server_addr/cred/queryNamespaces/9e2ada95-0602-4b4a-a2c6-532294fe4120. params. key:value):
     * "parentId":23
     * @return a list of Namespace serialized JSON if successful, null if unsuccessful
     * example:
     * [
     *     {
     *         "nsId": 528,
     *         "nsName": "folder1",
     *         "nsParentId": 552,
     *         "userId": 313
     *     },
     *     {
     *         "nsId": 552,
     *         "nsName": "folder2",
     *         "nsParentId": 552,
     *         "userId": 313
     *     }
     * ]
     */
    @GetMapping("/queryNamespaces/{token}")
    public List<Namespace> queryNamespaces(@PathVariable String token, Integer parentId)
    {
        Integer userId = service.getUserId(token);
        if (userId == null)
            return null;
        if (service.getAuth(token).contains("R"))
            return service.queryNamespaces(userId, parentId);
        else
            return null;
    }

    /**
     * Function to get list of photo's information based on a parent namespace id
     * @param token the credential's UUID
     * @param nsId the parent namespace's id
     * example (http://server_addr/cred/queryPhotoList/9e2ada95-0602-4b4a-a2c6-532294fe4120. params. key:value):
     * "nsId":23
     * @return a list of serialized Photo objects if successful, null if not successful
     * example:
     *[
     *     {
     *         "photoId": 542,
     *         "photoName": "jiaran2",
     *         "uploadDate": "2024-02-08T21:18:23",
     *         "format": "jpg",
     *         "nsId": 521,
     *         "visibility": 0,
     *         "token": "4c3ffe95-3d14-45a7-b2de-aaae984c54b2",
     *         "photoUuid": "9d2ccc95-0112-4b11-a2c2-502252ff41e0",
     *         "userId": 512
     *     }, ...
     * ]
     */
    @GetMapping("/queryPhotoList/{token}")
    public List<Photo> queryPhotoList(@PathVariable String token, Integer nsId)
    {
        Integer userId = service.getUserId(token);
        if (userId == null)
            return null;
        if (service.getAuth(token).contains("R"))
            return service.queryPhotoList(userId, nsId);
        else
            return null;
    }
}

