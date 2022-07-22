package org.chengbing.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import io.swagger.annotations.ApiModel;
import lombok.Data;
import lombok.EqualsAndHashCode;

/**
 * @Author: Harold澂冰
 * @Date: 2022/7/22 11:12
 */
@Data
@EqualsAndHashCode(callSuper = false)
@ApiModel(value="Setting Object", description="")
public class Setting {
    private static final long serialVersionUID = 1L;

    @TableId(value = "setting_id", type = IdType.AUTO)
    private Integer settingId;

    private String settingName;

    private String settingValue;

    private String note;

    public Setting(String settingName, String settingValue) {
        this.settingName = settingName;
        this.settingValue = settingValue;
    }

    public Setting(String settingName, String settingValue, String note) {
        this.settingName = settingName;
        this.settingValue = settingValue;
        this.note = note;
    }

    public Setting(Integer settingId, String settingName, String settingValue, String note) {
        this.settingId = settingId;
        this.settingName = settingName;
        this.settingValue = settingValue;
        this.note = note;
    }
}
